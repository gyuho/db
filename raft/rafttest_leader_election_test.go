package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

func Test_raft_campaign_candidate(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	msg := raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		To:   1,
		From: 1,
	}
	rnd.sendToMailbox(msg)
	rnd.Step(msg)

	rnd.assertNodeState(raftpb.NODE_STATE_CANDIDATE)
	if rnd.term != 1 {
		t.Fatalf("term expected 1, got %d", rnd.term)
	}
}

// (etcd raft.TestCampaignWhileLeader)

// (etcd raft.TestLeaderElection)
func Test_raft_leader_election(t *testing.T) {
	tests := []struct {
		fakeNetwork *fakeNetwork
		wNodeState  raftpb.NODE_STATE
	}{
		{newFakeNetwork(nil, nil, nil), raftpb.NODE_STATE_LEADER},
		{newFakeNetwork(nil, nil, noOpBlackHole), raftpb.NODE_STATE_LEADER},
		{newFakeNetwork(nil, noOpBlackHole, noOpBlackHole), raftpb.NODE_STATE_CANDIDATE},

		// quorum is 3
		{newFakeNetwork(nil, noOpBlackHole, noOpBlackHole, nil), raftpb.NODE_STATE_CANDIDATE},
		{newFakeNetwork(nil, noOpBlackHole, noOpBlackHole, nil, nil), raftpb.NODE_STATE_LEADER},

		// with higher terms than first node
		{newFakeNetwork(nil, newTestRaftNodeWithTerms(1), newTestRaftNodeWithTerms(2), newTestRaftNodeWithTerms(1, 3), nil), raftpb.NODE_STATE_FOLLOWER},

		// with higher terms including quorum, higest term in <quorum() nodes
		// out-of-range index from leader
		{newFakeNetwork(newTestRaftNodeWithTerms(1), nil, newTestRaftNodeWithTerms(2), newTestRaftNodeWithTerms(1), nil), raftpb.NODE_STATE_LEADER},
	}

	for i, tt := range tests {
		stepNode := tt.fakeNetwork.allStateMachines[1].(*raftNode)
		if stepNode.term != 0 {
			t.Fatalf("#%d: term expected 0, got %d", i, stepNode.term)
		}

		// to trigger election to 1
		tt.fakeNetwork.stepFirstFrontMessage(raftpb.Message{
			Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
			From: 1,
			To:   1,
		})

		if stepNode.state != tt.wNodeState {
			t.Fatalf("#%d: node state expected %q, got %q", i, tt.wNodeState, stepNode.state)
		}

		if stepNode.term != 1 { // should have increased
			t.Fatalf("#%d: term expected 1, got %d", i, stepNode.term)
		}
	}
}

// (etcd raft.TestSingleNodeCandidate)
func Test_raft_leader_election_single_node(t *testing.T) {
	fn := newFakeNetwork(nil)

	fn.stepFirstFrontMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})

	rnd1 := fn.allStateMachines[1].(*raftNode)
	if rnd1.state != raftpb.NODE_STATE_LEADER {
		t.Fatalf("rnd1 state expected %q, got %q", raftpb.NODE_STATE_LEADER, rnd1.state)
	}
}

func Test_raft_leader_election_checkQuorum_candidate(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd3 := newTestRaftNode(3, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd1.checkQuorum = true
	rnd2.checkQuorum = true
	rnd3.checkQuorum = true

	fn := newFakeNetwork(rnd1, rnd2, rnd3)

	// (Raft §4.2.3 Disruptive servers, p.42)
	//
	// "if a server receives a RequestVote request within the minimum election timeout
	// of hearing from a current leader, it does not update its term or grant its vote."
	//
	// this helps avoid disruptions from servers with old configuration
	//
	// If checkQuorum is true, leader checks if quorum of cluster are active for every election timeout
	// (if rnd.allProgresses[id].RecentActive {activeN++}).
	// And leader maintains 'Progress.RecentActive' for every incoming message from follower.
	// Now, if quorum is not active, leader reverts back to follower.
	//
	// Leader sends internal check-quorum message to trigger quorum-check
	// for every election timeout (raftNode.tickFuncLeaderHeartbeatTimeout).
	//
	// So if checkQuorum is true and election timeout has not happened yet,
	// then leader is guaranteed to have been in contact with quorum within
	// the last election timeout, as a valid leader.
	// So it shouldn't increase its term.
	//
	// SO, it's ok to to reject vote request

	// for i := 0; i < rnd1.electionTimeoutTickNum; i++ {
	// 	rnd1.tickFunc()
	// }

	fn.stepFirstFrontMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})

	if rnd1.state != raftpb.NODE_STATE_CANDIDATE {
		t.Fatalf("rnd1 state expected %q, got %q", raftpb.NODE_STATE_CANDIDATE, rnd1.state)
	}
}

// (etcd raft.TestLeaderElectionWithCheckQuorum)
func Test_raft_leader_election_checkQuorum_leader(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd3 := newTestRaftNode(3, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd1.checkQuorum = true
	rnd2.checkQuorum = true
	rnd3.checkQuorum = true

	fn := newFakeNetwork(rnd1, rnd2, rnd3)

	// time out rnd2, so that rnd2 now does not ignore the vote request
	// and rnd2 becomes the follower. And rnd1 will start campaign, and rnd2
	// will be able to vote for rnd1 to be a leader
	for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
		rnd2.tickFunc()
	}

	// rnd1 will start campaign and become the leader
	fn.stepFirstFrontMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

	// rnd1 check quorum is true, and passed election timeout
	// so it won't ignore the vote request, and then revert back to follower
	for i := 0; i < rnd1.electionTimeoutTickNum; i++ {
		rnd1.tickFunc()
	}

	// rnd2 check quorum is true, and passed election timeout
	// so it won't ignore the vote request, and then revert back to follower
	for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
		rnd2.tickFunc()
	}

	// rnd3 will start campaign and become the leader
	fn.stepFirstFrontMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 3,
		To:   3,
	})
	rnd1.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_LEADER)
}

// (etcd raft.TestLeaderSupersedingWithCheckQuorum)
func Test_raft_leader_election_checkQuorum_leader_ignore_vote(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd3 := newTestRaftNode(3, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd1.checkQuorum = true
	rnd2.checkQuorum = true
	rnd3.checkQuorum = true

	fn := newFakeNetwork(rnd1, rnd2, rnd3)

	// func (rnd *raftNode) pastElectionTimeout() bool {
	// 	return rnd.electionTimeoutElapsedTickNum >= rnd.randomizedElectionTimeoutTickNum
	// }
	rnd2.randomizedElectionTimeoutTickNum = rnd2.electionTimeoutTickNum + 1 // to prevent rnd2 from election timeout
	for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
		rnd2.tickFunc()
	}

	// rnd1 will start campaign and become the leader
	fn.stepFirstFrontMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

	// rnd3 will start campaign
	// but the vote requests will be rejected by rnd2
	// not yet timing out its election timeout
	fn.stepFirstFrontMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 3,
		To:   3,
	})

	// current leader rnd1 should ignore the vote request from rnd2
	// because it's leader
	//
	// 1. not leader transfer
	// 2. not candidate
	// 3. checkQuorum is true
	//    election timeout has not happened
	//
	if rnd3.state != raftpb.NODE_STATE_CANDIDATE {
		t.Fatalf("rnd3 state expected %q, got %q", raftpb.NODE_STATE_CANDIDATE, rnd3.state)
	}

	// election timeout
	for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
		rnd2.tickFunc()
	}

	// rnd3 will start campaign and become the leader
	fn.stepFirstFrontMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 3,
		To:   3,
	})

	if rnd3.state != raftpb.NODE_STATE_LEADER {
		t.Fatalf("rnd3 state expected %q, got %q", raftpb.NODE_STATE_LEADER, rnd3.state)
	}
}

// (etcd raft.TestFreeStuckCandidateWithCheckQuorum)
func Test_raft_leader_election_checkQuorum_candidate_with_higher_term(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd3 := newTestRaftNode(3, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd1.checkQuorum = true
	rnd2.checkQuorum = true
	rnd3.checkQuorum = true

	fn := newFakeNetwork(rnd1, rnd2, rnd3)

	// time out rnd2, so that rnd2 now does not ignore the vote request
	// and rnd2 becomes the follower. And rnd1 will start campaign, and rnd2
	// will be able to vote for rnd1 to be a leader
	for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
		rnd2.tickFunc()
	}

	// rnd1 will start campaign and become the leader
	fn.stepFirstFrontMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

	// isolate leader (cut all outgoing, incoming connections)
	fn.isolate(1)

	// trigger election from 3
	fn.stepFirstFrontMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 3,
		To:   3,
	})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_CANDIDATE)
	if rnd2.term+1 != rnd3.term {
		t.Fatalf("rnd2 expected term increase %d, got %d", rnd2.term+1, rnd3.term)
	}

	// trigger election from 3, again for safety
	fn.stepFirstFrontMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 3,
		To:   3,
	})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_CANDIDATE)
	if rnd2.term+2 != rnd3.term {
		t.Fatalf("rnd2 expected term increase %d, got %d", rnd2.term+2, rnd3.term)
	}

	// recover all dropped connections
	fn.recoverAll()

	// was-isolated leader now sends heartbeat to now-new-candidate
	fn.stepFirstFrontMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT,
		From: 1,
		To:   3,
	})

	// leader finds out the candidate with higher term
	// and reverts back to follower?
	rnd1.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_CANDIDATE)
	if rnd3.term != rnd1.term {
		t.Fatalf("rnd2 term expected %d, got %d", rnd1.term, rnd3.term)
	}
}

// (etcd raft.TestNonPromotableVoterWithCheckQuorum)
