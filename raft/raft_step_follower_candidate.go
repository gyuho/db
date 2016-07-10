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
		raftLogger.Infof("candidate %x received vote from %x at term %d", rnd.id, fromID, rnd.term)
	} else {
		raftLogger.Infof("candidate %x received vote-rejection from %x at term %d", rnd.id, fromID, rnd.term)
	}

	if _, ok := rnd.votedFrom[fromID]; !ok {
		rnd.votedFrom[fromID] = voted
	} else { // ???
		raftLogger.Panicf("candidate %x received duplicate votes from %x (voted %v)", rnd.id, fromID, voted)
	}

	grantedN := 0
	for _, voted := range rnd.votedFrom {
		if voted {
			grantedN++
		}
	}

	return grantedN
}

// (etcd raft.raft.handleHeartbeat)
func (rnd *raftNode) followerRespondToLeaderHeartbeat(msg raftpb.Message) {
	rnd.storageRaftLog.commitTo(msg.SenderCurrentCommittedIndex)
	rnd.sendToMailbox(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT,
		To:   msg.From,
	})
}

// (etcd raft.raft.campaign)
func (rnd *raftNode) followerBecomeCandidateAndStartCampaign() {
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
			"candidate %x [last log index=%d | last log term=%d] is sending vote requests to %x at term %d",
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
func (rnd *raftNode) followerRestoreSnapshotFromLeader(msg raftpb.Message) {
	if rnd.id == rnd.leaderID {
		raftLogger.Panicf("followerRestoreSnapshotFromLeader must be called by follower [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
	}

	snapMetaIndex, snapMetaTerm := msg.Snapshot.Metadata.Index, msg.Snapshot.Metadata.Term

	raftLogger.Infof("follower %x [committed index=%d] is restoring snapshot from leader %x [index=%d | term=%d]",
		rnd.id, rnd.storageRaftLog.committedIndex, rnd.leaderID, snapMetaIndex, snapMetaTerm,
	)

	if rnd.followerRestoreSnapshot(msg.Snapshot) {
		raftLogger.Infof("follower %x [committed index=%d] restored snapshot from leader %x [index=%d | term=%d]",
			rnd.id, rnd.storageRaftLog.committedIndex, rnd.leaderID, snapMetaIndex, snapMetaTerm,
		)
		rnd.sendToMailbox(raftpb.Message{
			Type:     raftpb.MESSAGE_TYPE_INTERNAL_RESPONSE_TO_LEADER_REQUEST_SNAPSHOT,
			To:       msg.From,
			LogIndex: rnd.storageRaftLog.lastIndex(),
		})
		return
	}

	raftLogger.Infof("follower %x [committed index=%d] ignored snapshot from leader %x [index=%d | term=%d]",
		rnd.id, rnd.storageRaftLog.committedIndex, rnd.leaderID, snapMetaIndex, snapMetaTerm,
	)
	rnd.sendToMailbox(raftpb.Message{
		Type:     raftpb.MESSAGE_TYPE_INTERNAL_RESPONSE_TO_LEADER_REQUEST_SNAPSHOT,
		To:       msg.From,
		LogIndex: rnd.storageRaftLog.committedIndex,
	})
}

// followerRestoreSnapshot returns true iff snapshot index is greater than committed index
// and the snapshot contains an entries of log and term that does not exist in the current follower.
//
// (etcd raft.raft.restore)
func (rnd *raftNode) followerRestoreSnapshot(snap raftpb.Snapshot) bool {
	if rnd.storageRaftLog.committedIndex >= snap.Metadata.Index {
		return false
	}

	if rnd.storageRaftLog.matchTerm(snap.Metadata.Index, snap.Metadata.Term) {
		raftLogger.Infof(`

	follower %x [committed index=%d | last index=%d | last term=%d]
	fast-forwarded commit
	from leader snapshot [index=%d | term=%d]

`, rnd.id, rnd.storageRaftLog.committedIndex, rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm(),
			snap.Metadata.Index, snap.Metadata.Term,
		)
		rnd.storageRaftLog.commitTo(snap.Metadata.Index)
		return false
	}

	raftLogger.Infof(`

	follower %x [committed index=%d | last index=%d | last term=%d]
	is restoring snapshot its state
	from leader snapshot [index=%d | term=%d]

`, rnd.id, rnd.storageRaftLog.committedIndex, rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm(),
		snap.Metadata.Index, snap.Metadata.Term,
	)

	rnd.storageRaftLog.restoreIncomingSnapshot(snap)

	raftLogger.Infof("follower %x is now initializing its progresses of peers", rnd.id)
	rnd.allProgresses = make(map[uint64]*Progress)
	for _, id := range snap.Metadata.ConfigState.IDs {
		matchIndex := uint64(0)
		if id == rnd.id {
			matchIndex = rnd.storageRaftLog.lastIndex()
		}
		nextIndex := rnd.storageRaftLog.lastIndex() + 1

		rnd.updateProgress(id, matchIndex, nextIndex)
		raftLogger.Infof("follower %x restored progress of %x %s", rnd.id, id, rnd.allProgresses[id])
	}

	return true
}

// (etcd raft.raft.becomeFollower)
func (rnd *raftNode) becomeFollower(term, leaderID uint64) {

}

// (etcd raft.raft.becomeCandidate)
func (rnd *raftNode) becomeCandidate() {

}
