package raft

import (
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
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("tickFuncLeaderHeartbeatTimeout must be called by leader [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
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
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("leaderSendHeartbeatTo must be called by leader [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
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
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("leaderReplicateHeartbeatRequests must be called by leader [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
	}

	for id := range rnd.allProgresses {
		if id == rnd.id { // OR rnd.leaderID
			continue
		}
		rnd.leaderSendHeartbeatTo(id)
		rnd.allProgresses[id].resume() // pr.Paused = false
	}
}

// leaderMaybeCommit tries to commit with the mid index of
// its progresses' match indexes.
//
// (etcd raft.raft.maybeCommit)
func (rnd *raftNode) leaderMaybeCommit() bool {
	matchIndexSlice := make(uint64Slice, 0, len(rnd.allProgresses))
	for id := range rnd.allProgresses {
		matchIndexSlice = append(matchIndexSlice, rnd.allProgresses[id].MatchIndex)
	}
	sort.Sort(sort.Reverse(matchIndexSlice))
	indexToCommit := matchIndexSlice[len(rnd.allProgresses)]

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
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("leaderSendAppendOrSnapshot must be called by leader [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
	}

	followerProgress := rnd.allProgresses[targetID]
	if followerProgress.isPaused() {
		raftLogger.Infof("leader %x skips sending heartbeat to paused follower %x", rnd.id, targetID)
		return
	}

	msg := raftpb.Message{
		To: targetID,
	}

	term, errTerm := rnd.storageRaftLog.term(followerProgress.NextIndex - 1) // term of leader
	entries, errEntries := rnd.storageRaftLog.entries(followerProgress.NextIndex, rnd.maxEntryNumPerMsg)

	if errTerm == nil || errEntries == nil {
		msg.Type = raftpb.MESSAGE_TYPE_LEADER_REQUEST_APPEND
		msg.LogIndex = followerProgress.NextIndex - 1
		msg.LogTerm = term
		msg.Entries = entries
		msg.SenderCurrentCommittedIndex = rnd.storageRaftLog.committedIndex
		if len(entries) > 0 {
			switch followerProgress.State {
			case raftpb.PROGRESS_STATE_PROBE:
				followerProgress.pause()

				// when 'leaderReplicateHeartbeatRequests', resume again
				// rnd.allProgresses[id].resume() // pr.Paused = false

			case raftpb.PROGRESS_STATE_REPLICATE:
				followerProgress.optimisticUpdate(entries[len(entries)-1].Index)
				followerProgress.inflights.add(entries[len(entries)-1].Index)

			default:
				raftLogger.Panicf("leader %x cannot send appends to follower %x of unhandled state %s", rnd.id, targetID, followerProgress)
			}
		}

	} else { // error if entries had been compacted in leader's logs
		msg.Type = raftpb.MESSAGE_TYPE_LEADER_REQUEST_SNAPSHOT

		raftLogger.Infof("leader %x now needs to send snapshot to follower %x [term error=%q | entries error=%q]", rnd.id, targetID, errTerm, errEntries)
		if !followerProgress.RecentActive {
			raftLogger.Infof("leader %x cancels snapshotting to follower %x [recent active=%v]", rnd.id, targetID, followerProgress.RecentActive)
			return
		}

		snapshot, err := rnd.storageRaftLog.snapshot()
		if err != nil {
			if err == ErrSnapshotTemporarilyUnavailable {
				raftLogger.Infof("leader %x failed to send snapshot to follower %x (%v)", rnd.id, targetID, err)
				return
			}
			raftLogger.Panicf("leader %x failed to send snapshot to follower %x (%v)", rnd.id, targetID, err)
		}

		if raftpb.IsEmptySnapshot(snapshot) {
			raftLogger.Panicf("leader %x returned empty snapshot", rnd.id)
		}

		msg.Snapshot = snapshot

		followerProgress.becomeSnapshot(snapshot.Metadata.Index)
		raftLogger.Infof(`

	leader %x [committed index=%d | first index=%d]
	stopped sending appends and is now sending snapshot [index=%d | term=%d]
	to follower %x %s

`, rnd.id, rnd.storageRaftLog.committedIndex, rnd.storageRaftLog.firstIndex(),
			snapshot.Metadata.Index, snapshot.Metadata.Term,
			targetID, followerProgress,
		)
	}

	raftLogger.Infof("leader %x is sending %q to follower %x in mailbox", rnd.id, msg.Type, msg.To)
	rnd.sendToMailbox(msg)
}

// leaderReplicateAppendRequests replicates append requests to its followers.
//
// (etcd raft.raft.bcastAppend)
func (rnd *raftNode) leaderReplicateAppendRequests() {
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("leaderReplicateAppendRequests must be called by leader [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
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
	storageLastIndex := rnd.storageRaftLog.lastIndex()
	for idx := range entries {
		entries[idx].Index = storageLastIndex + 1 + uint64(idx)
		entries[idx].Term = rnd.term
	}
	rnd.storageRaftLog.appendToStorageUnstable(entries...)

	rnd.allProgresses[rnd.id].maybeUpdate(rnd.storageRaftLog.lastIndex())

	rnd.leaderMaybeCommit()
}

// (etcd raft.raft.sendTimeoutNow)
func (rnd *raftNode) leaderForceFollowerElectionTimeout(targetID uint64) {
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("leaderForceFollowerElectionTimeout must be called by leader [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
	}

	rnd.sendToMailbox(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT,
		To:   targetID,
	})
}

// (etcd raft.raft.stepLeader)
func stepLeader(rnd *raftNode, msg raftpb.Message) {
	switch msg.Type {
	case raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_TO_SEND_HEARTBEAT:

	case raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_TO_CHECK_QUORUM:

	case raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER:

	case raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE:

	case raftpb.MESSAGE_TYPE_READ_LEADER_CURRENT_COMMITTED_INDEX_REQUEST:

	}
}

// (etcd raft.raft.becomeLeader)
func (rnd *raftNode) becomeLeader() {
	if rnd.state == raftpb.NODE_STATE_FOLLOWER {
		raftLogger.Panicf("follower %x cannot be leader without going through candidate state", rnd.id)
	}
	rnd.stepFunc = stepLeader
}
