package raft

import (
	"math"
	"sort"

	"github.com/gyuho/db/raft/raftpb"
)

// hasLeader returns true if there is a valid leader in the cluster.
//
// (etcd raft.raft.hasLeader)
func (rnd *raftNode) hasLeader() bool {
	return rnd.leaderID != NoNodeID
}

// checkQuorumActive returns true if the quorum of the cluster
// is active in the view of the local raft state machine.
//
// (etcd raft.raft.checkQuorumActive)
func (rnd *raftNode) checkQuorumActive() bool {
	activeN := 0
	for id := range rnd.allProgresses {
		if id == rnd.id {
			activeN++ // self is always active
			continue
		}

		if rnd.allProgresses[id].RecentActive {
			activeN++
		}

		// and resets the RecentActive
		rnd.allProgresses[id].RecentActive = false
	}

	return activeN >= rnd.quorum()
}

// tickFuncLeaderHeartbeatTimeout triggers an internal message to leader,
// so that leader can send out heartbeats to its followers. And if the
// election times out and 'checkQuorum' is true, it sends 'checkQuorum'
// message to its followers.
//
// (etcd raft.raft.tickHeartbeat)
func (rnd *raftNode) tickFuncLeaderHeartbeatTimeout() {
	rnd.heartbeatTimeoutElapsedTickNum++
	rnd.electionTimeoutElapsedTickNum++

	if rnd.electionTimeoutElapsedTickNum >= rnd.electionTimeoutTickNum {
		rnd.electionTimeoutElapsedTickNum = 0
		if rnd.checkQuorum {
			rnd.Step(raftpb.Message{
				Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CHECK_QUORUM,
				From: rnd.id,
			})
		}

		if rnd.state == raftpb.NODE_STATE_LEADER && rnd.leaderTransfereeID != NoNodeID {
			rnd.stopLeaderTransfer()
		}
	}

	if rnd.state != raftpb.NODE_STATE_LEADER {
		return
	}

	if rnd.heartbeatTimeoutElapsedTickNum >= rnd.heartbeatTimeoutTickNum {
		rnd.heartbeatTimeoutElapsedTickNum = 0
		rnd.Step(raftpb.Message{
			Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT,
			From: rnd.id,
		})
	}
}

// leaderSendHeartbeatTo sends an empty append RPC as a heartbeat to its followers.
//
// (etcd raft.raft.sendHeartbeat)
func (rnd *raftNode) leaderSendHeartbeatTo(targetID uint64) {
	var (
		matched         = rnd.allProgresses[targetID].MatchIndex
		commitInStorage = rnd.storageRaftLog.committedIndex
		committedIndex  = minUint64(matched, commitInStorage)
	)
	rnd.sendToMailbox(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT,
		To:   targetID,
		SenderCurrentCommittedIndex: committedIndex,
	})
}

// leaderReplicateHeartbeatRequests replicates heartbeats to its followers.
//
// (etcd raft.raft.bcastHeartbeat)
func (rnd *raftNode) leaderReplicateHeartbeatRequests() {
	for id := range rnd.allProgresses {
		if id == rnd.id { // OR rnd.leaderID
			continue
		}
		rnd.leaderSendHeartbeatTo(id)
		rnd.allProgresses[id].resume() // pr.Paused = false
	}
}

// leaderMaybeCommitWithQuorumMatchIndex tries to commit with quorum
// index of its progresses' match indexes. For example, if given [5, 5, 4],
// it tries to commit with 5 because quorum of cluster shares that match index.
//
// (Raft §3.5 Log replication, p.18)
// A log entry is committed once the leader has replicated the entry on quorum of cluster.
// It commits all preceding entries in the leader's log, including the ones from previous leaders.
//
// (etcd raft.raft.maybeCommit)
func (rnd *raftNode) leaderMaybeCommitWithQuorumMatchIndex() bool {
	matchIndexSlice := make(uint64Slice, 0, len(rnd.allProgresses))
	for id := range rnd.allProgresses {
		matchIndexSlice = append(matchIndexSlice, rnd.allProgresses[id].MatchIndex)
	}
	sort.Sort(sort.Reverse(matchIndexSlice))
	indexToCommit := matchIndexSlice[len(rnd.allProgresses)/2]

	// maybeCommit is only successful if 'indexToCommit' is greater than current 'committedIndex'
	// and the current term of 'indexToCommit' matches the 'termToCommit', without ErrCompacted.
	return rnd.storageRaftLog.maybeCommit(indexToCommit, rnd.currentTerm)
}

// leaderSendAppendOrSnapshot sends:
//   i)  LEADER_APPEND_REQUEST
//   OR
//   ii) LEADER_SNAPSHOT_REQUEST
//
// (etcd raft.raft.sendAppend)
func (rnd *raftNode) leaderSendAppendOrSnapshot(targetID uint64) {
	followerProgress := rnd.allProgresses[targetID]
	if followerProgress.isPaused() {
		raftLogger.Debugf("%s SKIPS append/snapshot to paused follower %x", rnd.describe(), targetID)
		return
	}

	msg := raftpb.Message{
		To: targetID,
	}

	term, errTerm := rnd.storageRaftLog.term(followerProgress.NextIndex - 1) // term of leader
	entries, errEntries := rnd.storageRaftLog.entries(followerProgress.NextIndex, rnd.maxEntryNumPerMsg)

	if errTerm == nil && errEntries == nil {
		msg.Type = raftpb.MESSAGE_TYPE_LEADER_APPEND
		msg.LogIndex = followerProgress.NextIndex - 1
		msg.LogTerm = term
		msg.Entries = entries
		msg.SenderCurrentCommittedIndex = rnd.storageRaftLog.committedIndex

		if len(msg.Entries) > 0 {
			switch followerProgress.State {
			case raftpb.PROGRESS_STATE_PROBE:
				followerProgress.pause()
				//
				// 'leaderReplicateHeartbeatRequests' will resume again
				// rnd.allProgresses[id].resume()

			case raftpb.PROGRESS_STATE_REPLICATE:
				lastIndex := msg.Entries[len(msg.Entries)-1].Index
				followerProgress.optimisticUpdate(lastIndex)
				followerProgress.inflights.add(lastIndex)

			default:
				raftLogger.Panicf("%s cannot send appends to follower %x of unhandled state %s", rnd.describe(), targetID, followerProgress)
			}
		}

	} else { // error if entries had been compacted in leader's logs
		msg.Type = raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT

		raftLogger.Infof(`

	%s
	TRIES TO SEND %q
	to FOLLOWER %x
	[term error='%v' | entries error='%v']

`, rnd.describeLong(), msg.Type, targetID, errTerm, errEntries)

		if !followerProgress.RecentActive {
			raftLogger.Infof("%s cancels snapshotting to FOLLOWER %x [recent active=%v]", rnd.describe(), targetID, followerProgress.RecentActive)
			return
		}

		snapshot, err := rnd.storageRaftLog.snapshot()
		if err != nil {
			if err == ErrSnapshotTemporarilyUnavailable {
				raftLogger.Infof("%s failed to send snapshot to FOLLOWER %x (%v)", rnd.describe(), targetID, err)
				return
			}
			raftLogger.Panicf("%s failed to send snapshot to FOLLOWER %x (%v)", rnd.describe(), targetID, err)
		}

		if raftpb.IsEmptySnapshot(snapshot) {
			raftLogger.Panicf("%s returned empty snapshot", rnd.describe())
		}

		msg.Snapshot = snapshot

		followerProgress.becomeSnapshot(snapshot.Metadata.Index)
		raftLogger.Infof(`

	%s
	SENDS %q [index=%d | term=%d]
	to FOLLOWER %x
	%s

`, rnd.describeLong(), msg.Type, snapshot.Metadata.Index, snapshot.Metadata.Term, targetID, followerProgress)

	}

	raftLogger.Debugf("%s SENDS %q to FOLLOWER %x in mailbox", rnd.describe(), msg.Type, msg.To)
	rnd.sendToMailbox(msg)
}

// leaderReplicateAppendRequests replicates append requests to its followers.
//
// (etcd raft.raft.bcastAppend)
func (rnd *raftNode) leaderReplicateAppendRequests() {
	for id := range rnd.allProgresses {
		if id == rnd.id { // OR rnd.leaderID
			continue
		}
		rnd.leaderSendAppendOrSnapshot(id)
	}
}

// (etcd raft.raft.appendEntry)
func (rnd *raftNode) leaderAppendEntriesToLeader(entries ...raftpb.Entry) {
	storageLastIndex := rnd.storageRaftLog.lastIndex()

	for idx := range entries {
		entries[idx].Index = storageLastIndex + 1 + uint64(idx)
		entries[idx].Term = rnd.currentTerm
	}

	rnd.storageRaftLog.appendToStorageUnstable(entries...)

	rnd.allProgresses[rnd.id].maybeUpdateAndResume(rnd.storageRaftLog.lastIndex())

	// leaderMaybeCommitWithQuorumMatchIndex tries to commit with quorum
	// index of its progresses' match indexes. For example, if given [5, 5, 4],
	// it tries to commit with 5 because quorum of cluster shares that match index.
	rnd.leaderMaybeCommitWithQuorumMatchIndex()
}

// (etcd raft.raft.sendTimeoutNow)
func (rnd *raftNode) leaderForceFollowerElectionTimeout(targetID uint64) {
	rnd.sendToMailbox(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT,
		To:   targetID,
	})
}

// (etcd raft.raft.becomeLeader)
func (rnd *raftNode) becomeLeader() {
	// cannot be leader without going through candidate state
	rnd.assertUnexpectedNodeState(raftpb.NODE_STATE_FOLLOWER)

	oldState := rnd.state

	rnd.resetWithTerm(rnd.currentTerm)
	rnd.leaderID = rnd.id
	rnd.state = raftpb.NODE_STATE_LEADER

	rnd.stepFunc = stepLeader
	rnd.tickFunc = rnd.tickFuncLeaderHeartbeatTimeout

	// get all uncommitted entries
	entries, err := rnd.storageRaftLog.entries(rnd.storageRaftLog.committedIndex+1, math.MaxUint64)
	if err != nil {
		raftLogger.Panicf("%s returned unexpected error (%v) while getting uncommitted entries", rnd.describe(), err)
	}

	for i := range entries {
		if entries[i].Type != raftpb.ENTRY_TYPE_CONFIG_CHANGE {
			continue
		}
		if rnd.pendingConfigExist {
			raftLogger.Panicf("%s has uncommitted duplicate configuration change entry (%+v)", rnd.describe(), entries[i])
		}
		rnd.pendingConfigExist = true
	}

	// (Raft §3.4 Leader election, p.16)
	//
	// When it becomes leader, it needs to send empty append-entries RPC call (heartbeat) to its
	// followers to establish its authority and prevent new elections.
	//
	//
	// (Raft §6.4 Processing read-only queries more efficiently, p.72)
	//
	// Leader Completeness Property guarantees that leader contains all committed entries.
	// But the leader may not know which entries are committed, especially at the beginning
	// of new term with new leader. To find out, leader needs to commit an entry in that term.
	// Raft makes each leader commit a blank no-op entry at the start of its term.
	// Once this no-op entris get committed, the leader committed index will be at least as large
	// as other peers in that term.
	//
	rnd.leaderAppendEntriesToLeader(raftpb.Entry{Data: nil})

	raftLogger.Infof("%s just transitioned from %q", rnd.describe(), oldState)
}

// (etcd raft.raft.stepLeader)
func stepLeader(rnd *raftNode, msg raftpb.Message) {
	rnd.assertNodeState(raftpb.NODE_STATE_LEADER)

	// (Raft §3.5 Log replication, p.17)
	//
	// Once leader gets elected, it starts serving client requests. Each request contains
	// a command to be executed by replicated state machines. Leader appends this command
	// to its log as a new entry, and send append RPCs to its followers to replicate them.
	//
	// Leader only commits a log entry only if the entry has been replicated in quorum of cluster.
	// Then leader applies the entry and returns the results to clients.
	// When follower finds out the entry is committed, the follower also applies the entry
	// to its local state machine.
	//
	// Leader forces followers to duplicate its own logs. Followers overwrites any conflicting
	// entries. Leader sends append RPCs by keeping the 'nextIndex' of each follower. And followers
	// will respond to these requests with reject hints of its last index, if needed.
	//
	//
	// If checkQuorum is true, leader checks if quorum of cluster are active for every election timeout.
	// Leader sends internal check-quorum message to trigger quorum-check
	// for every election timeout (raftNode.tickFuncLeaderHeartbeatTimeout).
	// Now, if quorum is not active, leader reverts back to follower.

	// leader to take action, or receive response
	switch msg.Type {
	case raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT: // pb.MsgBeat
		rnd.leaderReplicateHeartbeatRequests() // resume all progresses
		return

	case raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CHECK_QUORUM: // pb.MsgCheckQuorum
		if !rnd.checkQuorumActive() {
			raftLogger.Warningf("%s steps down to %q, because quorum is not active", rnd.describe(), raftpb.NODE_STATE_FOLLOWER)
			rnd.becomeFollower(rnd.currentTerm, NoNodeID) // becomeFollower(term, leader)
		}
		return

	case raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER: // pb.MsgProp
		if len(msg.Entries) == 0 {
			raftLogger.Panicf("%s got empty proposal", rnd.describe())
		}

		if _, ok := rnd.allProgresses[rnd.id]; !ok {
			// this node was removed from configuration while serving as leader
			// drop any new proposals
			raftLogger.Infof("%s was removed from configuration while serving as leader (dropping new proposals)", rnd.describe())
			return
		}

		if rnd.leaderTransfereeID != NoNodeID {
			raftLogger.Infof("%s is in progress of transferring its leadership to %x (dropping new proposals)", rnd.describe(), rnd.leaderTransfereeID)
			return
		}

		for i := range msg.Entries {
			if msg.Entries[i].Type == raftpb.ENTRY_TYPE_CONFIG_CHANGE {
				if rnd.pendingConfigExist {
					msg.Entries[i] = raftpb.Entry{Type: raftpb.ENTRY_TYPE_NORMAL}
				}
				rnd.pendingConfigExist = true
			}
		}

		rnd.leaderAppendEntriesToLeader(msg.Entries...)
		rnd.leaderReplicateAppendRequests()
		return

	case raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE: // pb.MsgVote

		// (Raft §3.4 Leader election, p.17)
		//
		// Raft randomizes election timeouts to minimize split votes or two candidates.
		// And each server votes for candidate on first-come base.
		// This randomized retry approach elects a leader rapidly, more obvious and understandable.
		//
		// leader or candidate rejects vote-requests from another server.

		raftLogger.Infof(`

	%s
	RECEIVED %s
	(GOT VOTE-REQUEST FROM, and REJECT-VOTED FOR %x)

`, rnd.describeLong(), raftpb.DescribeMessageLong(msg), msg.From)

		rnd.sendToMailbox(raftpb.Message{
			Type:   raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE,
			To:     msg.From,
			Reject: true,
		})
		return

	case raftpb.MESSAGE_TYPE_READ_INDEX: // pb.MsgReadIndex
		logIndex := uint64(0)
		if rnd.checkQuorum {
			logIndex = rnd.storageRaftLog.committedIndex
		}
		rnd.sendToMailbox(raftpb.Message{
			Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_READ_INDEX,
			To:       msg.From,
			LogIndex: logIndex,
			Entries:  msg.Entries,
		})
		return
	}

	followerProgress, ok := rnd.allProgresses[msg.From]
	if !ok {
		raftLogger.Debugf("%s has no progress of follower %x", rnd.describe(), msg.From)
		return
	}

	switch msg.Type {
	case raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT: // pb.MsgHeartbeatResp
		followerProgress.RecentActive = true

		if followerProgress.State == raftpb.PROGRESS_STATE_REPLICATE && followerProgress.inflights.full() {
			raftLogger.Debugf("%s frees the first inflight message of follower %x", rnd.describe(), msg.From)
			followerProgress.inflights.freeFirstOne()
			//
			// [10, 20, 30]
			// even if we free the first one 10, when we process 30, it will process 10 ~ 30.
		}

		if rnd.storageRaftLog.lastIndex() > followerProgress.MatchIndex {
			rnd.leaderSendAppendOrSnapshot(msg.From)
		}

	case raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND: // pb.MsgAppResp
		followerProgress.RecentActive = true

		switch msg.Reject {
		case true:
			raftLogger.Infof(`

	%s
	RECEIVED %s
	(LEADER APPEND-REQUEST is REJECTED!)

`, rnd.describeLong(), raftpb.DescribeMessageLong(msg))

			if followerProgress.maybeDecreaseAndResume(msg.LogIndex, msg.RejectHintFollowerLogLastIndex) {
				raftLogger.Infof(`

	%s
	decreased the progress of FOLLOWER %x
	to %s

`, rnd.describeLong(), msg.From, followerProgress)

				if followerProgress.State == raftpb.PROGRESS_STATE_REPLICATE {
					followerProgress.becomeProbe()
				}
				rnd.leaderSendAppendOrSnapshot(msg.From) // retry
			}

		case false:
			wasPausedOrFull := followerProgress.isPaused()
			if followerProgress.maybeUpdateAndResume(msg.LogIndex) {
				switch followerProgress.State {
				case raftpb.PROGRESS_STATE_PROBE:
					followerProgress.becomeReplicate()

				case raftpb.PROGRESS_STATE_REPLICATE:
					// succeed, so free up to entries <= msg.LogIndex
					followerProgress.inflights.freeTo(msg.LogIndex)

				case raftpb.PROGRESS_STATE_SNAPSHOT:
					if followerProgress.needSnapshotAbort() { // pr.MatchIndex >= pr.PendingSnapshotIndex
						followerProgress.becomeProbe()
						raftLogger.Infof("%s is stopping snapshot to follower %x, and resetting progress to %s", rnd.describe(), msg.From, followerProgress)
					}
				}

				// leaderMaybeCommitWithQuorumMatchIndex tries to commit with quorum
				// index of its progresses' match indexes. For example, if given [5, 5, 4],
				// it tries to commit with 5 because quorum of cluster shares that match index.
				if rnd.leaderMaybeCommitWithQuorumMatchIndex() {
					rnd.leaderReplicateAppendRequests()
				} else if wasPausedOrFull { // if maybeCommit didn't succeed, since it's now resumed, so send now
					rnd.leaderSendAppendOrSnapshot(msg.From)
				}

				if rnd.leaderTransfereeID == msg.From && rnd.storageRaftLog.lastIndex() == followerProgress.MatchIndex {
					raftLogger.Infof("%s force-election-times out follower %x for leadership transfer", rnd.describe(), msg.From)
					rnd.leaderForceFollowerElectionTimeout(msg.From)
				}
			}
		}

	case raftpb.MESSAGE_TYPE_INTERNAL_RESPONSE_TO_LEADER_SNAPSHOT: // pb.MsgSnapStatus
		if followerProgress.State != raftpb.PROGRESS_STATE_SNAPSHOT {
			return
		}

		switch msg.Reject {
		case false:
			followerProgress.becomeProbe()
			followerProgress.pause()
			raftLogger.Infof(`

	%s
	SENT SNAPSHOT and RECEIVED RESPONSE
	from FOLLOWER %x
	%s

`, rnd.describeLong(), msg.From, followerProgress)
			//
			// 'leaderReplicateHeartbeatRequests' will resume again
			// rnd.allProgresses[id].resume()

		case true:
			followerProgress.snapshotFailed() // set pending snapshot index to 0
			followerProgress.becomeProbe()
			followerProgress.pause()
			raftLogger.Infof(`

	%s
	SENT SNAPSHOT BUT GOT REJECTED
	from FOLLOWER %x
	%s

`, rnd.describeLong(), msg.From, followerProgress)
			//
			// 'leaderReplicateHeartbeatRequests' will resume again
			// rnd.allProgresses[id].resume()
		}

	case raftpb.MESSAGE_TYPE_INTERNAL_LEADER_CANNOT_CONNECT_TO_FOLLOWER: // pb.MsgUnreachable

		if followerProgress.State == raftpb.PROGRESS_STATE_REPLICATE {
			followerProgress.becomeProbe()
		}

		raftLogger.Warningf(`

	%s
	RECEIVED %s
	(LEADER FAILED TO SEND message to FOLLOWER %x)

`, rnd.describeLong(), raftpb.DescribeMessageLong(msg), msg.From)

	case raftpb.MESSAGE_TYPE_INTERNAL_LEADER_TRANSFER: // pb.MsgTransferLeader
		lastLeaderTransfereeID := rnd.leaderTransfereeID
		leaderTransfereeID := msg.From

		if lastLeaderTransfereeID != NoNodeID {
			if lastLeaderTransfereeID == leaderTransfereeID {
				raftLogger.Infof("%s is already transferring its leadership to follower %x (ignores this request)", rnd.describe(), leaderTransfereeID)
				return
			}

			rnd.stopLeaderTransfer()
			raftLogger.Infof(`

	%s
	CANCELLED leadership transfer to FOLLOWER %x
	(%x got a new leadership transfer request to FOLLOWER %x)

`, rnd.describeLong(), lastLeaderTransfereeID, rnd.id, leaderTransfereeID)
		}

		if rnd.id == leaderTransfereeID {
			raftLogger.Infof("%s is already a leader, so ignores leadership transfer request", rnd.describe())
			return
		}

		rnd.leaderTransfereeID = leaderTransfereeID
		rnd.electionTimeoutElapsedTickNum = 0
		raftLogger.Infof("%s starts transferring its leadership to follower %x", rnd.describe(), rnd.leaderTransfereeID)

		if rnd.storageRaftLog.lastIndex() == followerProgress.MatchIndex {
			raftLogger.Infof("%s force-election-times out follower, leader-transferee %x, which already has up-to-date log", rnd.describe(), leaderTransfereeID)
			rnd.leaderForceFollowerElectionTimeout(leaderTransfereeID)
		} else {
			rnd.leaderSendAppendOrSnapshot(leaderTransfereeID)
		}
	}
}
