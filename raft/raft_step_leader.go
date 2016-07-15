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

// leaderCheckQuorumActive returns true if the quorum of the cluster
// is active in the view of the local raft state machine.
//
// (etcd raft.raft.checkQuorumActive)
func (rnd *raftNode) leaderCheckQuorumActive() bool {
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
	if rnd.state != raftpb.NODE_STATE_LEADER {
		raftLogger.Panicf("rnd.state must be %q, got %q", raftpb.NODE_STATE_LEADER, rnd.state)
	}
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("tickFuncLeaderHeartbeatTimeout must be called by %q [id=%x | leader id=%x]", rnd.state, rnd.id, rnd.leaderID)
	}

	rnd.heartbeatTimeoutElapsedTickNum++
	rnd.electionTimeoutElapsedTickNum++

	if rnd.electionTimeoutElapsedTickNum >= rnd.electionTimeoutTickNum {
		rnd.electionTimeoutElapsedTickNum = 0
		if rnd.leaderCheckQuorum {
			rnd.Step(raftpb.Message{
				Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_TO_CHECK_QUORUM,
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
			Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_TO_SEND_HEARTBEAT,
			From: rnd.id,
		})
	}
}

// leaderSendHeartbeatTo sends an empty append RPC as a heartbeat to its followers.
//
// (etcd raft.raft.sendHeartbeat)
func (rnd *raftNode) leaderSendHeartbeatTo(targetID uint64) {
	if rnd.state != raftpb.NODE_STATE_LEADER {
		raftLogger.Panicf("rnd.state must be %q, got %q", raftpb.NODE_STATE_LEADER, rnd.state)
	}
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("leaderSendHeartbeatTo must be called by %q [id=%x | leader id=%x]", rnd.state, rnd.id, rnd.leaderID)
	}

	// committedIndex is min(to.matched, raftNode.committedIndex).
	//
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
	if rnd.state != raftpb.NODE_STATE_LEADER {
		raftLogger.Panicf("rnd.state must be %q, got %q", raftpb.NODE_STATE_LEADER, rnd.state)
	}
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("leaderReplicateHeartbeatRequests must be called by %q [id=%x | leader id=%x]", rnd.state, rnd.id, rnd.leaderID)
	}

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
	if rnd.state != raftpb.NODE_STATE_LEADER {
		raftLogger.Panicf("rnd.state must be %q, got %q", raftpb.NODE_STATE_LEADER, rnd.state)
	}

	matchIndexSlice := make(uint64Slice, 0, len(rnd.allProgresses))
	for id := range rnd.allProgresses {
		matchIndexSlice = append(matchIndexSlice, rnd.allProgresses[id].MatchIndex)
	}
	sort.Sort(sort.Reverse(matchIndexSlice))
	indexToCommit := matchIndexSlice[len(rnd.allProgresses)/2]

	// maybeCommit is only successful if 'indexToCommit' is greater than current 'committedIndex'
	// and the current term of 'indexToCommit' matches the 'termToCommit', without ErrCompacted.
	return rnd.storageRaftLog.maybeCommit(indexToCommit, rnd.term)
}

// leaderSendAppendOrSnapshot sends:
//   i)  LEADER_APPEND_REQUEST
//   OR
//   ii) LEADER_SNAPSHOT_REQUEST
//
// (etcd raft.raft.sendAppend)
func (rnd *raftNode) leaderSendAppendOrSnapshot(targetID uint64) {
	if rnd.state != raftpb.NODE_STATE_LEADER {
		raftLogger.Panicf("rnd.state must be %q, got %q", raftpb.NODE_STATE_LEADER, rnd.state)
	}
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("leaderSendAppendOrSnapshot must be called by %q [id=%x | leader id=%x]", rnd.state, rnd.id, rnd.leaderID)
	}

	followerProgress := rnd.allProgresses[targetID]
	if followerProgress.isPaused() {
		raftLogger.Infof("%q %x skips append/snapshot to paused follower %x", rnd.state, rnd.id, targetID)
		return
	}

	msg := raftpb.Message{
		To: targetID,
	}

	term, errTerm := rnd.storageRaftLog.term(followerProgress.NextIndex - 1) // term of leader
	entries, errEntries := rnd.storageRaftLog.entries(followerProgress.NextIndex, rnd.maxEntryNumPerMsg)

	if errTerm == nil || errEntries == nil {
		msg.Type = raftpb.MESSAGE_TYPE_APPEND_FROM_LEADER

		msg.LogIndex = followerProgress.NextIndex - 1
		msg.LogTerm = term
		msg.Entries = entries
		msg.SenderCurrentCommittedIndex = rnd.storageRaftLog.committedIndex
		if len(entries) > 0 {
			switch followerProgress.State {
			case raftpb.PROGRESS_STATE_PROBE:
				followerProgress.pause()
				//
				// 'leaderReplicateHeartbeatRequests' will resume again
				// rnd.allProgresses[id].resume()

			case raftpb.PROGRESS_STATE_REPLICATE:
				followerProgress.optimisticUpdate(entries[len(entries)-1].Index)
				followerProgress.inflights.add(entries[len(entries)-1].Index)

			default:
				raftLogger.Panicf("%q %x cannot send appends to follower %x of unhandled state %s", rnd.state, rnd.id, targetID, followerProgress)
			}
		}

	} else { // error if entries had been compacted in leader's logs
		msg.Type = raftpb.MESSAGE_TYPE_SNAPSHOT_FROM_LEADER

		raftLogger.Infof("%q %x now needs to send snapshot to follower %x [term error=%q | entries error=%q]", rnd.state, rnd.id, targetID, errTerm, errEntries)
		if !followerProgress.RecentActive {
			raftLogger.Infof("%q %x cancels snapshotting to follower %x [recent active=%v]", rnd.state, rnd.id, targetID, followerProgress.RecentActive)
			return
		}

		snapshot, err := rnd.storageRaftLog.snapshot()
		if err != nil {
			if err == ErrSnapshotTemporarilyUnavailable {
				raftLogger.Infof("%q %x failed to send snapshot to follower %x (%v)", rnd.state, rnd.id, targetID, err)
				return
			}
			raftLogger.Panicf("%q %x failed to send snapshot to follower %x (%v)", rnd.state, rnd.id, targetID, err)
		}

		if raftpb.IsEmptySnapshot(snapshot) {
			raftLogger.Panicf("%q %x returned empty snapshot", rnd.state, rnd.id)
		}

		msg.Snapshot = snapshot

		followerProgress.becomeSnapshot(snapshot.Metadata.Index)
		raftLogger.Infof(`

	%q %x [committed index=%d | first index=%d]
	stopped sending appends and is now sending snapshot [index=%d | term=%d]
	to follower %x %s

`, rnd.state, rnd.id, rnd.storageRaftLog.committedIndex, rnd.storageRaftLog.firstIndex(),
			snapshot.Metadata.Index, snapshot.Metadata.Term,
			targetID, followerProgress,
		)
	}

	raftLogger.Infof("%q %x sends %q to follower %x in mailbox", rnd.state, rnd.id, msg.Type, msg.To)
	rnd.sendToMailbox(msg)
}

// leaderReplicateAppendRequests replicates append requests to its followers.
//
// (etcd raft.raft.bcastAppend)
func (rnd *raftNode) leaderReplicateAppendRequests() {
	if rnd.state != raftpb.NODE_STATE_LEADER {
		raftLogger.Panicf("rnd.state must be %q, got %q", raftpb.NODE_STATE_LEADER, rnd.state)
	}
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("leaderReplicateAppendRequests must be called by %q [id=%x | leader id=%x]", rnd.state, rnd.id, rnd.leaderID)
	}

	for id := range rnd.allProgresses {
		if id == rnd.id { // OR rnd.leaderID
			continue
		}
		rnd.leaderSendAppendOrSnapshot(id)
	}
}

// (etcd raft.raft.appendEntry)
func (rnd *raftNode) leaderAppendEntriesToLeader(entries ...raftpb.Entry) {
	if rnd.state != raftpb.NODE_STATE_LEADER {
		raftLogger.Panicf("rnd.state must be %q, got %q", raftpb.NODE_STATE_LEADER, rnd.state)
	}

	storageLastIndex := rnd.storageRaftLog.lastIndex()
	for idx := range entries {
		entries[idx].Index = storageLastIndex + 1 + uint64(idx)
		entries[idx].Term = rnd.term
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
	if rnd.state != raftpb.NODE_STATE_LEADER {
		raftLogger.Panicf("rnd.state must be %q, got %q", raftpb.NODE_STATE_LEADER, rnd.state)
	}
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("leaderForceFollowerElectionTimeout must be called by %q [id=%x | leader id=%x]", rnd.state, rnd.id, rnd.leaderID)
	}

	rnd.sendToMailbox(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT,
		To:   targetID,
	})
}

// (etcd raft.raft.stepLeader)
func stepLeader(rnd *raftNode, msg raftpb.Message) {
	if rnd.state != raftpb.NODE_STATE_LEADER {
		raftLogger.Panicf("rnd.state must be %q, got %q", raftpb.NODE_STATE_LEADER, rnd.state)
	}

	// leader to take action, or receive response
	switch msg.Type {
	case raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_TO_SEND_HEARTBEAT: // pb.MsgBeat
		rnd.leaderReplicateHeartbeatRequests()
		return

	case raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_TO_CHECK_QUORUM: // pb.MsgCheckQuorum
		if !rnd.leaderCheckQuorumActive() {
			raftLogger.Warningf("%q %x is stepping down to %q since quorum of cluster is not active", rnd.state, rnd.id, raftpb.NODE_STATE_FOLLOWER)
			rnd.becomeFollower(rnd.term, NoNodeID) // becomeFollower(term, leader)
		}
		return

	case raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER: // pb.MsgProp
		if len(msg.Entries) == 0 {
			raftLogger.Panicf("%q %x got empty proposal", rnd.state, rnd.id)
		}

		if _, ok := rnd.allProgresses[rnd.id]; !ok {
			// ???
			// this node was removed from configuration while serving as leader
			// drop any new proposals
			raftLogger.Infof("%q %x was removed from configuration while serving as leader (dropping new proposals)", rnd.state, rnd.id)
			return
		}

		if rnd.leaderTransfereeID != NoNodeID {
			raftLogger.Infof("%q %x [term=%d] is in progress of transferring its leadership to %x (dropping new proposals)",
				rnd.state, rnd.id, rnd.term, rnd.leaderTransfereeID)
			return
		}

		for i := range msg.Entries {
			if msg.Entries[i].Type == raftpb.ENTRY_TYPE_CONFIG_CHANGE {
				if rnd.pendingConfigExist { // ???
					msg.Entries[i] = raftpb.Entry{Type: raftpb.ENTRY_TYPE_NORMAL}
				}
				rnd.pendingConfigExist = true
			}
		}

		rnd.leaderAppendEntriesToLeader(msg.Entries...)
		rnd.leaderReplicateAppendRequests()
		return

	case raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE: // pb.MsgVote
		raftLogger.Infof(`

	%q %x [log term=%d | log index=%d | voted for %x]
	is rejecting to vote
	for %x [message log index=%d | message log term=%d]

`, rnd.state, rnd.id, rnd.storageRaftLog.lastTerm(), rnd.storageRaftLog.lastIndex(), rnd.votedFor,
			msg.From, msg.LogIndex, msg.LogTerm,
		)

		rnd.sendToMailbox(raftpb.Message{
			Type:   raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE,
			To:     msg.From,
			Reject: true,
		})
		return

	case raftpb.MESSAGE_TYPE_READ_LEADER_CURRENT_COMMITTED_INDEX: // pb.MsgReadIndex
		logIndex := uint64(0)
		if rnd.leaderCheckQuorum {
			logIndex = rnd.storageRaftLog.committedIndex
		}
		rnd.sendToMailbox(raftpb.Message{
			Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_READ_LEADER_CURRENT_COMMITTED_INDEX,
			LogIndex: logIndex,
			Entries:  msg.Entries,
		})
		return
	}

	followerProgress, ok := rnd.allProgresses[msg.From]
	if !ok {
		raftLogger.Infof("%q %x has no progress of follower %x", rnd.state, rnd.id, msg.From)
		return
	}

	switch msg.Type {
	case raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT: // pb.MsgHeartbeatResp
		followerProgress.RecentActive = true

		if followerProgress.State == raftpb.PROGRESS_STATE_REPLICATE && followerProgress.inflights.full() {
			raftLogger.Infof("%q %x frees the first inflight message of follower %x", rnd.state, rnd.id, msg.From)
			followerProgress.inflights.freeFirstOne()
			//
			// [10, 20, 30]
			// even if we free the first one 10, when we process 30, it will process 10 ~ 30.
		}

		if rnd.storageRaftLog.lastIndex() > followerProgress.MatchIndex {
			rnd.leaderSendAppendOrSnapshot(msg.From)
		}

	case raftpb.MESSAGE_TYPE_RESPONSE_TO_APPEND_FROM_LEADER: // pb.MsgAppResp
		followerProgress.RecentActive = true

		switch msg.Reject {
		case false:
			wasPaused := followerProgress.isPaused()
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
						raftLogger.Infof("%q %x is stopping snapshot to follower %x, and resetting progress to %s", rnd.state, rnd.id, msg.From, followerProgress)
					}
				}

				// leaderMaybeCommitWithQuorumMatchIndex tries to commit with quorum
				// index of its progresses' match indexes. For example, if given [5, 5, 4],
				// it tries to commit with 5 because quorum of cluster shares that match index.
				if rnd.leaderMaybeCommitWithQuorumMatchIndex() {
					rnd.leaderReplicateAppendRequests()
				} else if wasPaused { // now resumed, so send now
					rnd.leaderSendAppendOrSnapshot(msg.From)
				}

				if rnd.leaderTransfereeID == msg.From && rnd.storageRaftLog.lastIndex() == followerProgress.MatchIndex {
					raftLogger.Infof("%q %x force-election-times out follower %x for leadership transfer", rnd.state, rnd.id, msg.From)
					rnd.leaderForceFollowerElectionTimeout(msg.From)
				}
			}

		case true:
			raftLogger.Infof(`

	%q %x [log index=%d | log term=%d]
	received rejection in %q
	from follower %x [requested log index=%d | follower reject hint last log index=%d]

`, rnd.state, rnd.id, rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm(), msg.Type,
				msg.From, msg.LogIndex, msg.RejectHintFollowerLogLastIndex)

			if followerProgress.maybeDecreaseAndResume(msg.LogIndex, msg.RejectHintFollowerLogLastIndex) {
				raftLogger.Infof("%q %x decreased the progress of follower %x to %s", rnd.state, rnd.id, msg.From, followerProgress)
				if followerProgress.State == raftpb.PROGRESS_STATE_REPLICATE {
					followerProgress.becomeProbe()
				}
				rnd.leaderSendAppendOrSnapshot(msg.From) // retry
			}
		}

	case raftpb.MESSAGE_TYPE_INTERNAL_RESPONSE_TO_SNAPSHOT_FROM_LEADER: // pb.MsgSnapStatus
		if followerProgress.State != raftpb.PROGRESS_STATE_SNAPSHOT {
			return
		}

		switch msg.Reject {
		case false:
			followerProgress.becomeProbe()
			followerProgress.pause()
			raftLogger.Infof("%q %x sent snapshot and received response from follower %x %s", rnd.state, rnd.id, msg.From, followerProgress)
			//
			// 'leaderReplicateHeartbeatRequests' will resume again
			// rnd.allProgresses[id].resume()

		case true:
			followerProgress.snapshotFailed()
			followerProgress.becomeProbe()
			followerProgress.pause()
			raftLogger.Infof("%q %x sent snapshot but got rejected from follower %x %s", rnd.state, rnd.id, msg.From, followerProgress)
			//
			// 'leaderReplicateHeartbeatRequests' will resume again
			// rnd.allProgresses[id].resume()
		}

	case raftpb.MESSAGE_TYPE_INTERNAL_LEADER_CANNOT_CONNECT_TO_FOLLOWER: // pb.MsgUnreachable
		if followerProgress.State == raftpb.PROGRESS_STATE_REPLICATE {
			followerProgress.becomeProbe()
		}
		raftLogger.Infof(`

	%q %x cannot connect to follower %x
	leader failed to send message:
	%s

`, rnd.state, rnd.id, msg.From, raftpb.DescribeMessage(msg))

	case raftpb.MESSAGE_TYPE_INTERNAL_TRANSFER_LEADER: // pb.MsgTransferLeader
		lastLeaderTransfereeID := rnd.leaderTransfereeID
		leaderTransfereeID := msg.From

		if rnd.id == leaderTransfereeID {
			raftLogger.Infof("%q %x is already a leader, so ignores leadership transfer request", rnd.state, rnd.id)
			return
		}

		if lastLeaderTransfereeID != NoNodeID {
			if lastLeaderTransfereeID == leaderTransfereeID {
				raftLogger.Infof("%q %x is already transferring its leadership to follower %x (ignores this request)", rnd.state, rnd.id, leaderTransfereeID)
				return
			}
			rnd.stopLeaderTransfer()
			raftLogger.Infof(`

	%q %x has just cancelled leadership transfer to follower %x
	(got a new leadership transfer request to follower %x)

`, rnd.state, rnd.id, lastLeaderTransfereeID, leaderTransfereeID)
		}

		rnd.leaderTransfereeID = leaderTransfereeID
		rnd.electionTimeoutElapsedTickNum = 0
		raftLogger.Infof("%q %x starts transferring its leadership to follower %x", rnd.state, rnd.id, rnd.leaderTransfereeID)

		if rnd.storageRaftLog.lastIndex() == followerProgress.MatchIndex {
			raftLogger.Infof("%q %x force-election-times out follower, leader-transferee %x, which already has up-to-date log", rnd.state, rnd.id, leaderTransfereeID)
			rnd.leaderForceFollowerElectionTimeout(leaderTransfereeID)
		} else {
			rnd.leaderSendAppendOrSnapshot(leaderTransfereeID)
		}
	}
}

// (etcd raft.raft.becomeLeader)
func (rnd *raftNode) becomeLeader() {
	if rnd.state == raftpb.NODE_STATE_FOLLOWER {
		raftLogger.Panicf("%q %x cannot be leader without going through candidate state", rnd.state, rnd.id)
	}
	oldState := rnd.state

	rnd.resetWithTerm(rnd.term)
	rnd.leaderID = rnd.id
	rnd.state = raftpb.NODE_STATE_LEADER

	rnd.stepFunc = stepLeader
	rnd.tickFunc = rnd.tickFuncLeaderHeartbeatTimeout

	// get all uncommitted entries
	entries, err := rnd.storageRaftLog.entries(rnd.storageRaftLog.committedIndex+1, math.MaxUint64)
	if err != nil {
		raftLogger.Panicf("%q %x returned unexpected error (%v) while getting uncommitted entries", rnd.state, rnd.id, err)
	}

	for i := range entries {
		if entries[i].Type != raftpb.ENTRY_TYPE_CONFIG_CHANGE {
			continue
		}
		if rnd.pendingConfigExist {
			raftLogger.Panicf("%q %x has uncommitted duplicate configuration change entry (%+v)", rnd.state, rnd.id, entries[i])
		}
		rnd.pendingConfigExist = true
	}

	// (Raft §3.4 Leader election, p.16)
	// When it becomes leader, it needs to send empty append-entries RPC call (heartbeat) to its
	// followers to establish its authority and prevent new elections.
	//
	// (Raft §6.4 Processing read-only queries more efficiently, p.72)
	// Leader Completeness Property guarantees that leader contains all committed entries.
	// But the leader may not know which entries are committed, especially at the beginning
	// of new term with new leader. To find out, leader needs to commit an entry in that term.
	// Raft makes each leader commit a blank no-op entry at the start of its term.
	// Once this no-op entris get committed, the leader committed index will be at least as large
	// as other peers in that term.
	//
	rnd.leaderAppendEntriesToLeader(raftpb.Entry{Data: nil})

	raftLogger.Infof("%q %x became %q at term %d", oldState, rnd.id, rnd.state, rnd.term)
}
