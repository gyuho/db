package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

func Test_raft_trigger_campaign_and_candidate(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	msg := raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, To: 1, From: 1}
	rnd.sendToMailbox(msg)
	rnd.Step(msg)

	rnd.assertNodeState(raftpb.NODE_STATE_CANDIDATE)
	if rnd.currentTerm != 1 {
		t.Fatalf("term expected 1, got %d", rnd.currentTerm)
	}
}

// (etcd raft.TestPromotable)
func Test_raft_promotableToLeader(t *testing.T) {
	tests := []struct {
		rnd         *raftNode
		wPromotable bool
	}{
		{newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory()), true},
		{newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory()), true},
		{newTestRaftNode(1, []uint64{}, 10, 1, NewStorageStableInMemory()), false},
		{newTestRaftNode(1, []uint64{2, 3}, 10, 1, NewStorageStableInMemory()), false},
	}
	for i, tt := range tests {
		if g := tt.rnd.promotableToLeader(); g != tt.wPromotable {
			t.Fatalf("#%d: promotable to leader expected %v, got %v", i, tt.wPromotable, g)
		}
	}
}

// (etcd raft.TestRecvMsgBeat)
func Test_raft_Step_trigger_leader_heartbeat(t *testing.T) {
	tests := []struct {
		currentState raftpb.NODE_STATE
		stepFunc     func(r *raftNode, msg raftpb.Message)
		msgToStep    raftpb.Message

		wMsgNum int
	}{
		{
			raftpb.NODE_STATE_LEADER,
			stepLeader,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT, From: 1, To: 1},

			2,
		},

		{ // candidate ignores MsgBeat (MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT)
			raftpb.NODE_STATE_CANDIDATE,
			stepCandidate,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT, From: 1, To: 1},

			0,
		},

		{ // follower ignores MsgBeat (MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT)
			raftpb.NODE_STATE_FOLLOWER,
			stepFollower,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT, From: 1, To: 1},

			0,
		},
	}

	for i, tt := range tests {
		rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
		rnd.storageRaftLog = &storageRaftLog{
			storageStable: &StorageStableInMemory{snapshotEntries: []raftpb.Entry{{}, {Index: 1, Term: 0}, {Index: 2, Term: 1}}},
		}
		rnd.currentTerm = 1

		rnd.state = tt.currentState
		rnd.stepFunc = tt.stepFunc

		rnd.Step(tt.msgToStep)

		msgs := rnd.readAndClearMailbox()
		if len(msgs) != tt.wMsgNum {
			t.Fatalf("#%d: message num expected %d, got %d", i, tt.wMsgNum, len(msgs))
		}
		for _, msg := range msgs {
			if msg.Type != raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT {
				t.Fatalf("msg.Type expected %q, got %q", raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT, msg.Type)
			}
		}
	}
}

// (etcd raft.TestCampaignWhileLeader)
func Test_raft_Step_trigger_campaign_while_leader(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())
	rnd.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

	rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})
	rnd.assertNodeState(raftpb.NODE_STATE_LEADER)

	oldTerm := rnd.currentTerm

	rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})
	rnd.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd.currentTerm != oldTerm {
		t.Fatalf("term should not be increased (expected %d, got %d)", oldTerm, rnd.currentTerm)
	}
}

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
		if stepNode.currentTerm != 0 {
			t.Fatalf("#%d: term expected 0, got %d", i, stepNode.currentTerm)
		}

		// to trigger election to 1
		tt.fakeNetwork.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})

		if stepNode.state != tt.wNodeState {
			t.Fatalf("#%d: node state expected %q, got %q", i, tt.wNodeState, stepNode.state)
		}

		if stepNode.currentTerm != 1 { // should have increased
			t.Fatalf("#%d: term expected 1, got %d", i, stepNode.currentTerm)
		}
	}
}

// (etcd raft.TestSingleNodeCandidate)
func Test_raft_leader_election_single_node(t *testing.T) {
	fn := newFakeNetwork(nil)

	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})

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

	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})

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
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})
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
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 3, To: 3})
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
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

	// rnd3 will start campaign
	// but the vote requests will be rejected by rnd2
	// not yet timing out its election timeout
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 3, To: 3})

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
	fn.stepFirstMessage(raftpb.Message{
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
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

	// isolate leader (cut all outgoing, incoming connections)
	fn.isolate(1)

	// trigger election from 3
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 3, To: 3})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_CANDIDATE)
	if rnd2.currentTerm+1 != rnd3.currentTerm {
		t.Fatalf("rnd2 expected term increase %d, got %d", rnd2.currentTerm+1, rnd3.currentTerm)
	}

	// trigger election from 3, again for safety
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 3, To: 3})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_CANDIDATE)
	if rnd2.currentTerm+2 != rnd3.currentTerm {
		t.Fatalf("rnd2 expected term increase %d, got %d", rnd2.currentTerm+2, rnd3.currentTerm)
	}

	// recover all dropped connections
	fn.recoverAll()

	// was-isolated leader now sends heartbeat to now-new-candidate
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT, From: 1, To: 3})

	// once candidate rnd3 gets the heartbeat from leader,
	// it must revert back to follower first
	//
	// And rnd3 handles heartbeat and responds with higher term
	// and then leader rnd1 gets the response with higher term,
	// so it reverts back to follower
	rnd1.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	if rnd3.currentTerm != rnd1.currentTerm {
		t.Fatalf("rnd2 term expected %d, got %d", rnd1.currentTerm, rnd3.currentTerm)
	}
}

// (etcd raft.TestNonPromotableVoterWithCheckQuorum)
func Test_raft_leader_election_checkQuorum_non_promotable_voter(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1}, 10, 1, NewStorageStableInMemory())

	rnd1.checkQuorum = true
	rnd2.checkQuorum = true

	fn := newFakeNetwork(rnd1, rnd2)

	// remove again, because it is updated inside 'newFakeNetwork'
	rnd2.deleteProgress(2)
	if rnd2.promotableToLeader() { // _, ok := rnd.allProgresses[rnd.id]
		t.Fatalf("rnd2 must not be promotable %s", rnd2.describe())
	}

	// time out rnd2
	for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
		rnd2.tickFunc()
	}

	// trigger campaign in rnd1
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leader must be 1, got %d", rnd1.leaderID)
	}
	if rnd2.leaderID != 1 {
		t.Fatalf("leader must be 1, got %d", rnd2.leaderID)
	}
}
