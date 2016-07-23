package raft

import (
	"reflect"
	"sort"
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
func Test_raft_paper_leader_broadcast_heartbeat(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory()) // heartbeat tick is 1
	rnd.becomeCandidate()
	rnd.becomeLeader()

	for i := 0; i < 10; i++ {
		rnd.leaderAppendEntriesToLeader(raftpb.Entry{Index: uint64(i) + 1})
	}

	// to trigger leader to send heartbeat
	for i := 0; i < rnd.heartbeatTimeoutTickNum; i++ {
		rnd.tickFunc()
	}
	// tickFunc
	//
	// if rnd.heartbeatTimeoutElapsedTickNum >= rnd.heartbeatTimeoutTickNum {
	// 	rnd.heartbeatTimeoutElapsedTickNum = 0
	// 	rnd.Step(raftpb.Message{
	// 		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT,
	// 		From: rnd.id,
	// 	})
	// }

	msgs := rnd.readAndClearMailbox()
	sort.Sort(messageSlice(msgs))

	wmsgs := []raftpb.Message{
		{Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT, From: 1, To: 2, SenderCurrentTerm: 1},
		{Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT, From: 1, To: 3, SenderCurrentTerm: 1},
	}
	if !reflect.DeepEqual(msgs, wmsgs) {
		t.Fatalf("messages expected %+v, got %+v", wmsgs, msgs)
	}
}

// (Raft §3.4 Leader election, p.16)
// (etcd raft.TestFollowerStartElection)
func Test_raft_paper_follower_start_election(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd.becomeFollower(1, 2) // term 1, leader is 2
	oldTerm := rnd.term

	// election timeout to trigger election from follower
	for i := 0; i < 2*rnd.electionTimeoutTickNum; i++ {
		rnd.tickFunc()
	}

	if rnd.term != oldTerm+1 {
		t.Fatalf("term should have increased to %d, got %d", oldTerm+1, rnd.term)
	}

	rnd.assertNodeState(raftpb.NODE_STATE_CANDIDATE)

	if !rnd.votedFrom[rnd.id] {
		t.Fatalf("should have voted for itself, got %+v", rnd.votedFrom)
	}

	msgs := rnd.readAndClearMailbox()
	sort.Sort(messageSlice(msgs))

	// follower is now candidate sending vote-requests
	wmsgs := []raftpb.Message{
		{Type: raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE, From: 1, To: 2, SenderCurrentTerm: 2},
		{Type: raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE, From: 1, To: 3, SenderCurrentTerm: 2},
	}
	if !reflect.DeepEqual(msgs, wmsgs) {
		t.Fatalf("messages expected %+v, got %+v", wmsgs, msgs)
	}
}

// (Raft §3.4 Leader election, p.16)
// (etcd raft.TestCandidateStartNewElection)
func Test_raft_paper_candidate_start_election(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd.becomeCandidate() // term 2
	oldTerm := rnd.term

	// election timeout to trigger another election from candidate
	// candidate starts a new election and increments its term
	for i := 0; i < 2*rnd.electionTimeoutTickNum; i++ {
		rnd.tickFunc()
	}

	if rnd.term != oldTerm+1 {
		t.Fatalf("term should have increased to %d, got %d", oldTerm+1, rnd.term)
	}

	rnd.assertNodeState(raftpb.NODE_STATE_CANDIDATE)

	if !rnd.votedFrom[rnd.id] {
		t.Fatalf("should have voted for itself, got %+v", rnd.votedFrom)
	}

	msgs := rnd.readAndClearMailbox()
	sort.Sort(messageSlice(msgs))

	// follower is now candidate sending vote-requests
	wmsgs := []raftpb.Message{
		{Type: raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE, From: 1, To: 2, SenderCurrentTerm: 2},
		{Type: raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE, From: 1, To: 3, SenderCurrentTerm: 2},
	}
	if !reflect.DeepEqual(msgs, wmsgs) {
		t.Fatalf("messages expected %+v, got %+v", wmsgs, msgs)
	}
}

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
