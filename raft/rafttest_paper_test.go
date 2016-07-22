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
func Test_raft_paper_candidate_update_term_from_Message(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd.becomeCandidate()

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

// (etcd raft.TestLeaderUpdateTermFromMessage)
func Test_raft_paper_leader_update_term_from_Message(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd.becomeCandidate()
	rnd.becomeLeader()

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

// (etcd raft.TestStartAsFollower)
//
// (Raft §3.3 Raft basics, p.14)
func Test_raft_paper_start_as_follower(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	if rnd.state != raftpb.NODE_STATE_FOLLOWER {
		t.Fatalf("state expected %q, got %q", raftpb.NODE_STATE_FOLLOWER, rnd.state)
	}
}

// (etcd raft.TestRejectStaleTermMessage)
//
// (Raft §3.3 Raft basics, p.15)
func Test_raft_paper_reject_stale_term_message(t *testing.T) {
	called := false
	stepFuncTest := func(rnd *raftNode, msg raftpb.Message) {
		called = true
	}

	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd.stepFunc = stepFuncTest
	rnd.loadHardState(raftpb.HardState{Term: 2})

	rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, SenderCurrentTerm: rnd.term - 1})

	if called {
		t.Fatal("message should have been rejected not calling stepFunc")
	}
}

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
