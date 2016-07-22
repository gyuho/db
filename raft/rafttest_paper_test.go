package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestFollowerUpdateTermFromMessage)
func Test_raft_paper_follower_update_term_from_Message(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd.becomeFollower(1, 2) // leaderID 2

	// case msg.SenderCurrentTerm > rnd.term:
	//
	// func (rnd *raftNode) sendToMailbox(msg raftpb.Message) {
	//   msg.From = rnd.id
	//
	rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, SenderCurrentTerm: 2})

	if rnd.term != 2 {
		t.Fatalf("term expected 2, got %d", rnd.term)
	}
	if rnd.state != raftpb.NODE_STATE_FOLLOWER {
		t.Fatalf("node state expected %q, got %q", raftpb.NODE_STATE_FOLLOWER, rnd.state)
	}
}

// (etcd raft.TestCandidateUpdateTermFromMessage)

// (etcd raft.TestLeaderUpdateTermFromMessage)

// (etcd raft.TestRejectStaleTermMessage)

// (etcd raft.TestStartAsFollower)

// (etcd raft.TestLeaderBcastBeat)

// (etcd raft.TestFollowerStartElection)

// (etcd raft.TestCandidateStartNewElection)

// (etcd raft.TestLeaderElectionInOneRoundRPC)

// (etcd raft.TestFollowerVote)

// (etcd raft.TestCandidateFallback)

// (etcd raft.TestFollowerElectionTimeoutRandomized)

// (etcd raft.TestCandidateElectionTimeoutRandomized)

// (etcd raft.TestFollowersElectioinTimeoutNonconflict)

// (etcd raft.TestCandidatesElectionTimeoutNonconflict)

// (etcd raft.TestLeaderStartReplication)

// (etcd raft.TestLeaderCommitEntry)

// (etcd raft.TestLeaderAcknowledgeCommit)

// (etcd raft.TestLeaderCommitPrecedingEntries)

// (etcd raft.TestFollowerCommitEntry)

// (etcd raft.TestFollowerCheckMsgApp)

// (etcd raft.TestFollowerAppendEntries)

// (etcd raft.TestLeaderSyncFollowerLog)

// (etcd raft.TestVoteRequest)

// (etcd raft.TestVoter)

// (etcd raft.TestLeaderOnlyCommitsLogFromCurrentTerm)
