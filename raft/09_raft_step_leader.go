package raft

import (
	"math"
	"sort"

	"github.com/gyuho/db/pkg/types"
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
func (rnd *raftNode) leaderSendHeartbeatTo(targetID uint64, ctx []byte) {
	var (
		matched         = rnd.allProgresses[targetID].MatchIndex
		commitInStorage = rnd.storageRaftLog.committedIndex
		committedIndex  = minUint64(matched, commitInStorage)
	)
	rnd.sendToMailbox(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT,
		To:   targetID,
		SenderCurrentCommittedIndex: committedIndex,

		Context: ctx, // for read-index
	})
}

// (etcd raft.raft.bcastHeartbeat)
func (rnd *raftNode) leaderSendHeartbeats() {
	lastCtx := rnd.readOnly.lastPendingRequestCtx()
	if len(lastCtx) == 0 {
		rnd.leaderSendHeartbeatsCtx(nil)
		return
	}
	rnd.leaderSendHeartbeatsCtx([]byte(lastCtx))
}

// (etcd raft.raft.bcastHeartbeatWithCtx)
func (rnd *raftNode) leaderSendHeartbeatsCtx(ctx []byte) {
	for id := range rnd.allProgresses {
		if id == rnd.id { // OR rnd.leaderID
			continue
		}
		rnd.leaderSendHeartbeatTo(id, ctx)
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
		raftLogger.Infof("%s skips append/snapshot to paused follower %s", rnd.describe(), types.ID(targetID))
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
				// 'leaderSendHeartbeats' will resume again
				// rnd.allProgresses[id].resume()

			case raftpb.PROGRESS_STATE_REPLICATE:
				lastIndex := msg.Entries[len(msg.Entries)-1].Index
				followerProgress.optimisticUpdate(lastIndex)
				followerProgress.inflights.add(lastIndex)

			default:
				raftLogger.Panicf("%s cannot send append to follower %s of unhandled state %s", rnd.describe(), types.ID(targetID), followerProgress)
			}
		}

	} else { // error if entries had been compacted in leader's logs
		msg.Type = raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT
		raftLogger.Infof("%s needs snapshot to %s (%v; %v)", rnd.describe(), types.ID(targetID), errTerm, errEntries)
		if !followerProgress.RecentActive {
			raftLogger.Infof("%s cancels snapshotting to follower %x [recent active=%v]", rnd.describe(), targetID, followerProgress.RecentActive)
			return
		}

		snapshot, err := rnd.storageRaftLog.snapshot()
		if err != nil {
			if err == ErrSnapshotTemporarilyUnavailable {
				raftLogger.Infof("%s failed snapshot to FOLLOWER %s (%v)", rnd.describe(), types.ID(targetID), err)
				return
			}
			raftLogger.Panicf("%s failed snapshot to FOLLOWER %s (%v)", rnd.describe(), types.ID(targetID), err)
		}

		if raftpb.IsEmptySnapshot(snapshot) {
			raftLogger.Panicf("%s returned empty snapshot", rnd.describe())
		}
		msg.Snapshot = snapshot
		followerProgress.becomeSnapshot(snapshot.Metadata.Index)
		raftLogger.Infof("%s sends snapshot to %s  [index=%d | term=%d]", rnd.describe(), types.ID(targetID), snapshot.Metadata.Index, snapshot.Metadata.Term)
	}

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

// (etcd raft.numOfPendingConf)
func countPendingConfigChange(entries []raftpb.Entry) int {
	n := 0
	for i := range entries {
		if entries[i].Type == raftpb.ENTRY_TYPE_CONFIG_CHANGE {
			n++
		}
	}
	return n
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

	nconf := countPendingConfigChange(entries)
	switch {
	case nconf > 1:
		raftLogger.Panicf("%s has uncommitted duplicate configuration change entry (%+v)", rnd.describe(), entries)
	case nconf == 1:
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

	raftLogger.Infof("%s transitioned from %q", rnd.describe(), oldState)
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
	case raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT:
		rnd.leaderSendHeartbeats() // resume all progresses
		return

	case raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CHECK_QUORUM:
		if !rnd.checkQuorumActive() {
			raftLogger.Warningf("%s becomes %q; quorum is not active", rnd.describe(), raftpb.NODE_STATE_FOLLOWER)
			rnd.becomeFollower(rnd.currentTerm, NoNodeID)
		}
		return

	case raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER:
		if len(msg.Entries) == 0 {
			raftLogger.Panicf("%s got empty proposal", rnd.describe())
		}

		if _, ok := rnd.allProgresses[rnd.id]; !ok {
			raftLogger.Infof("%s was removed from configuration while serving as leader; dropping proposal", rnd.describe())
			return
		}

		if rnd.leaderTransfereeID != NoNodeID {
			raftLogger.Infof("%s is transferring leadership to %s; dropping proposal", rnd.describe(), types.ID(rnd.leaderTransfereeID))
			return
		}

		for i := range msg.Entries {
			if msg.Entries[i].Type == raftpb.ENTRY_TYPE_CONFIG_CHANGE {
				if rnd.pendingConfigExist {
					raftLogger.Infof("proposal %q is ignored since there's pending unapplied configuration", msg.Entries[i].Type.String())
					msg.Entries[i] = raftpb.Entry{Type: raftpb.ENTRY_TYPE_NORMAL}
				}
				rnd.pendingConfigExist = true
			}
		}

		rnd.leaderAppendEntriesToLeader(msg.Entries...)
		rnd.leaderReplicateAppendRequests()
		return

	case raftpb.MESSAGE_TYPE_TRIGGER_READ_INDEX: // manually called from node
		if rnd.quorum() > 1 {
			switch rnd.readOnly.option {
			case ReadOnlySafe:
				rnd.readOnly.addRequest(msg, rnd.storageRaftLog.committedIndex)
				rnd.leaderSendHeartbeatsCtx(msg.Entries[0].Data)

			case ReadOnlyLeaseBased:
				logIndex := uint64(0)
				if rnd.checkQuorum {
					logIndex = rnd.storageRaftLog.committedIndex
				}
				if msg.From == uint64(0) || msg.From == rnd.id { // from local member(leader)
					rnd.readStates = append(rnd.readStates, ReadState{
						Index:      rnd.storageRaftLog.committedIndex,
						RequestCtx: msg.Entries[0].Data,
					})
				} else { // from follower
					rnd.sendToMailbox(raftpb.Message{
						Type:     raftpb.MESSAGE_TYPE_READ_INDEX_DATA,
						To:       msg.From,
						LogIndex: logIndex,
						Entries:  msg.Entries,
					})
				}
			}
		} else { // from local member(leader)
			rnd.readStates = append(rnd.readStates, ReadState{
				Index:      rnd.storageRaftLog.committedIndex,
				RequestCtx: msg.Entries[0].Data,
			})
		}
		return
	}

	followerProgress, ok := rnd.allProgresses[msg.From]
	if !ok {
		raftLogger.Infof("%s has no progress of follower %s; dropping %q", rnd.describe(), types.ID(msg.From), msg.Type)
		return
	}

	switch msg.Type {
	case raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT:
		followerProgress.RecentActive = true

		if followerProgress.State == raftpb.PROGRESS_STATE_REPLICATE && followerProgress.inflights.full() {
			followerProgress.inflights.freeFirstOne()
			//
			// [10, 20, 30]
			// even if we free the first one 10, when we process 30, it will process 10 ~ 30.
		}

		if rnd.storageRaftLog.lastIndex() > followerProgress.MatchIndex {
			rnd.leaderSendAppendOrSnapshot(msg.From)
		}

		if rnd.readOnly.option != ReadOnlySafe || len(msg.Context) == 0 {
			return
		}
		ackCount := rnd.readOnly.recvAck(msg)
		if ackCount < rnd.quorum() {
			return
		}

		rss := rnd.readOnly.advance(msg)
		for _, rs := range rss {
			req := rs.req
			if req.From == uint64(0) || req.From == rnd.id { // from local member(leader)
				rnd.readStates = append(rnd.readStates, ReadState{
					Index:      rs.index,
					RequestCtx: req.Entries[0].Data,
				})
			} else { // from follower
				rnd.sendToMailbox(raftpb.Message{
					Type:     raftpb.MESSAGE_TYPE_READ_INDEX_DATA,
					To:       req.From,
					LogIndex: rs.index,
					Entries:  req.Entries,
				})
			}
		}

	case raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND:
		followerProgress.RecentActive = true

		switch msg.Reject {
		case true:
			raftLogger.Infof("%s sent append-request, rejected from %s [hint=%d]", rnd.describe(), types.ID(msg.From), msg.RejectHintFollowerLogLastIndex)
			if followerProgress.maybeDecreaseAndResume(msg.LogIndex, msg.RejectHintFollowerLogLastIndex) {
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
						raftLogger.Infof("%s stops snapshot to follower %s; updates progress %q", rnd.describe(), types.ID(msg.From), followerProgress.State)
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
					raftLogger.Infof("%s force-election-times out follower %s(leadership transfer)", rnd.describe(), types.ID(msg.From))
					rnd.leaderForceFollowerElectionTimeout(msg.From)
				}
			}
		}

	case raftpb.MESSAGE_TYPE_INTERNAL_RESPONSE_TO_LEADER_SNAPSHOT:
		if followerProgress.State != raftpb.PROGRESS_STATE_SNAPSHOT {
			return
		}

		switch msg.Reject {
		case false:
			followerProgress.becomeProbe()
			followerProgress.pause()
			raftLogger.Infof("%s got snapshot-response approved from %s", rnd.describe(), types.ID(msg.From))
			//
			// 'leaderSendHeartbeats' will resume again
			// rnd.allProgresses[id].resume()

		case true:
			followerProgress.snapshotFailed() // set pending snapshot index to 0
			followerProgress.becomeProbe()
			followerProgress.pause()
			raftLogger.Infof("%s got snapshot-response rejected from %s", rnd.describe(), types.ID(msg.From))
			//
			// 'leaderSendHeartbeats' will resume again
			// rnd.allProgresses[id].resume()
		}

	case raftpb.MESSAGE_TYPE_INTERNAL_LEADER_CANNOT_CONNECT_TO_FOLLOWER:
		raftLogger.Warningf("%s cannot connect to %s", rnd.describe(), types.ID(msg.From))
		if followerProgress.State == raftpb.PROGRESS_STATE_REPLICATE {
			followerProgress.becomeProbe()
		}
		// only pause when we know for sure the probing would fail
		// OR
		// when we want to ensure one inflight
		//
		// if unreachable, probing should continue to probe to find
		// the follower when it's back online
		//
		// followerProgress.pause()

	case raftpb.MESSAGE_TYPE_TRANSFER_LEADER:
		lastLeaderTransfereeID := rnd.leaderTransfereeID
		leaderTransfereeID := msg.From

		if lastLeaderTransfereeID != NoNodeID {
			if lastLeaderTransfereeID == leaderTransfereeID {
				raftLogger.Infof("%s is already transferring leadership to follower %s; ignores leadership transfer request", rnd.describe(), types.ID(leaderTransfereeID))
				return
			}
			rnd.stopLeaderTransfer()
			raftLogger.Infof("%s cancelled leadership transfer to follower %s; now to %s", rnd.describe(), types.ID(lastLeaderTransfereeID), types.ID(leaderTransfereeID))
		}

		if rnd.id == leaderTransfereeID {
			raftLogger.Infof("%s is already leader; ignores leadership transfer request", rnd.describe())
			return
		}

		rnd.leaderTransfereeID = leaderTransfereeID
		rnd.electionTimeoutElapsedTickNum = 0
		raftLogger.Infof("%s starts leadership transfer to follower %s", rnd.describe(), types.ID(rnd.leaderTransfereeID))

		if rnd.storageRaftLog.lastIndex() == followerProgress.MatchIndex {
			raftLogger.Infof("%s force-election-times out transferee %s(with up-to-date log)", rnd.describe(), types.ID(leaderTransfereeID))
			rnd.leaderForceFollowerElectionTimeout(leaderTransfereeID)
		} else {
			rnd.leaderSendAppendOrSnapshot(leaderTransfereeID)
		}
	}
}
