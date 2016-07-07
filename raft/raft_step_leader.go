package raft

import "github.com/gyuho/db/raft/raftpb"

// hasLeader returns true if there is a valid leader in the cluster.
//
// (etcd raft.raft.hasLeader)
func (rnd *raftNode) hasLeader() bool {
	return rnd.leaderID != NoneNodeID
}

// leaderSendHeartbeatTo sends an empty append RPC as a heartbeat to its followers.
//
// (etcd raft.raft.sendHeartbeat)
func (rnd *raftNode) leaderSendHeartbeatTo(target uint64) {
	// committedIndex is min(to.matched, raftNode.committedIndex).
	//
	var (
		matched         = rnd.allProgresses[target].MatchIndex
		commitInStorage = rnd.storageRaftLog.committedIndex
		committedIndex  = minUint64(matched, commitInStorage)
	)
	rnd.sendToMailbox(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT_REQUEST,
		To:   target,
		CurrentCommittedIndex: committedIndex,
	})
}

// (etcd raft.raft.sendTimeoutNow)
func (rnd *raftNode) leaderForceFollowerElectionTimeout(target uint64) {
	rnd.sendToMailbox(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT,
		To:   target,
	})
}

func (rnd *raftNode) becomeLeader() {

}
