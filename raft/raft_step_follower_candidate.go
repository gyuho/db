package raft

import "github.com/gyuho/db/raft/raftpb"

// promotableToLeader return true if the local state machine can be promoted to leader.
//
// (etcd raft.raft.promotable)
func (rnd *raftNode) promotableToLeader() bool {
	_, ok := rnd.allProgresses[rnd.id]
	return ok
}

// tickFuncFollowerElectionTimeout triggers an internal message to leader,
// so that leader can send out heartbeats to its followers.
//
// (etcd raft.raft.tickElection)
func (rnd *raftNode) tickFuncFollowerElectionTimeout() {
	if rnd.id == rnd.leaderID {
		raftLogger.Panicf("tickFuncFollowerElectionTimeout must be called by follower [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
	}

	rnd.electionTimeoutElapsedTickNum++
	if rnd.promotableToLeader() && rnd.pastElectionTimeout() {
		rnd.electionTimeoutElapsedTickNum = 0
		rnd.Step(raftpb.Message{
			Type: raftpb.MESSAGE_TYPE_INTERNAL_CAMPAIGN_START,
			From: rnd.id,
		})
	}
}

// (etcd raft.raft.poll)
func (rnd *raftNode) candidateReceivedVoteFrom(fromID uint64, voted bool) int {
	if voted {
		raftLogger.Infof("%x received vote from %x at term %d", rnd.id, fromID, rnd.term)
	} else {
		raftLogger.Infof("%x received vote-rejection from %x at term %d", rnd.id, fromID, rnd.term)
	}

	if _, ok := rnd.votedFrom[fromID]; !ok {
		rnd.votedFrom[fromID] = voted
	} else { // ???
		raftLogger.Panicf("%x received duplicate votes from %x (voted %v)", rnd.id, fromID, voted)
	}

	grantedN := 0
	for _, voted := range rnd.votedFrom {
		if voted {
			grantedN++
		}
	}

	return grantedN
}

// (etcd raft.raft.campaign)
func (rnd *raftNode) followerStartCampaign() {
	rnd.becomeCandidate()

	// vote for itself, and then if voted from quorum, become leader
	if rnd.quorum() == rnd.candidateReceivedVoteFrom(rnd.id, true) {
		rnd.becomeLeader()
		return
	}

	// request vote from peers
	for id := range rnd.allProgresses {
		if id == rnd.id {
			continue
		}

		raftLogger.Infof(
			"%x [last log index=%d | last log term=%d] is sending vote requests to %x at term %d",
			rnd.id, rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm(), id, rnd.term,
		)
		rnd.sendToMailbox(raftpb.Message{
			Type:     raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE,
			To:       id,
			LogIndex: rnd.storageRaftLog.lastIndex(),
			LogTerm:  rnd.storageRaftLog.lastTerm(),
		})
	}
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

// (etcd raft.raft.becomeCandidate)
func (rnd *raftNode) becomeCandidate() {

}
