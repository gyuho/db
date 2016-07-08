package raft

import "github.com/gyuho/db/raft/raftpb"

// hasLeader returns true if there is a valid leader in the cluster.
//
// (etcd raft.raft.hasLeader)
func (rnd *raftNode) hasLeader() bool {
	return rnd.leaderID != NoneNodeID
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
				Type: raftpb.MESSAGE_TYPE_INTERNAL_CHECK_QUORUM,
				From: rnd.id,
			})
		}

		if rnd.state == raftpb.NODE_STATE_LEADER && rnd.leaderTransfereeID != NoneNodeID {
			rnd.stopLeaderTransfer()
		}
	}

	if rnd.state != raftpb.NODE_STATE_LEADER {
		return
	}

	if rnd.heartbeatTimeoutElapsedTickNum >= rnd.heartbeatTimeoutTickNum {
		rnd.heartbeatTimeoutElapsedTickNum = 0
		rnd.Step(raftpb.Message{
			Type: raftpb.MESSAGE_TYPE_INTERNAL_LEADER_SEND_HEARTBEAT,
			From: rnd.id,
		})
	}
}

// leaderSendHeartbeatTo sends an empty append RPC as a heartbeat to its followers.
//
// (etcd raft.raft.sendHeartbeat)
func (rnd *raftNode) leaderSendHeartbeatTo(target uint64) {
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("leaderSendHeartbeatTo must be called by leader [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
	}

	// committedIndex is min(to.matched, raftNode.committedIndex).
	//
	var (
		matched         = rnd.allProgresses[target].MatchIndex
		commitInStorage = rnd.storageRaftLog.committedIndex
		committedIndex  = minUint64(matched, commitInStorage)
	)
	rnd.sendToMailbox(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT,
		To:   target,
		CurrentCommittedIndex: committedIndex,
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

// leaderSendAppendOrSnapshot sends:
//   i)  LEADER_APPEND_REQUEST
//   OR
//   ii) LEADER_SNAPSHOT_REQUEST
//
// (etcd raft.raft.sendAppend)
func (rnd *raftNode) leaderSendAppendOrSnapshot(target uint64) {
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("leaderSendAppendOrSnapshot must be called by leader [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
	}

	// TODO
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
func (rnd *raftNode) leaderAppendEntries(entries ...raftpb.Entry) {
	storageLastIndex := rnd.storageRaftLog.lastIndex()
	for idx := range entries {
		entries[idx].Index = storageLastIndex + 1 + uint64(idx)
		entries[idx].Term = rnd.term
	}
	rnd.storageRaftLog.appendToStorageUnstable(entries...)

	// ???
	rnd.allProgresses[rnd.id].maybeUpdate(rnd.storageRaftLog.lastIndex())

	// ???
	rnd.maybeCommit()
}

// (etcd raft.raft.sendTimeoutNow)
func (rnd *raftNode) leaderForceFollowerElectionTimeout(target uint64) {
	if rnd.id != rnd.leaderID {
		raftLogger.Panicf("leaderForceFollowerElectionTimeout must be called by leader [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
	}

	rnd.sendToMailbox(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT,
		To:   target,
	})
}

// (etcd raft.raft.becomeLeader)
func (rnd *raftNode) becomeLeader() {

}
