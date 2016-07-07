package raft

import "github.com/gyuho/db/raft/raftpb"

// promotableToLeader return true if the local state machine can be promoted to leader.
//
// (etcd raft.raft.promotable)
func (rnd *raftNode) promotableToLeader() bool {
	_, ok := rnd.allProgresses[rnd.id]
	return ok
}

// (etcd raft.raft.campaign)
func (rnd *raftNode) followerCampaign() {
	rnd.becomeCandidate()
}

// (etcd raft.raft.handleSnapshot with raftpb.MsgSnap)
func (rnd *raftNode) handleSnapshotFromLeader(msg raftpb.Message) {
	if rnd.id == rnd.leaderID {
		raftLogger.Panicf("handleSnapshotFromLeader must be called by follower [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
	}

}

// (etcd raft.raft.restore)
func (rnd *raftNode) restoreFromSnapshot(snap raftpb.Snapshot) {
	if rnd.id == rnd.leaderID {
		raftLogger.Panicf("restoreFromSnapshot must be called by follower [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
	}

}

// (etcd raft.raft.becomeFollower)
func (rnd *raftNode) becomeFollower(term, leaderID uint64) {

}
