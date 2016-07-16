package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

func Test_raft_campaign_candidate(t *testing.T) {
	rnd := newRaftNode(&Config{
		ID:         1,
		allPeerIDs: []uint64{1, 2, 3},

		ElectionTickNum:         5,
		HeartbeatTimeoutTickNum: 1,

		LeaderCheckQuorum: false,
		StorageStable:     NewStorageStableInMemory(),
		MaxEntryNumPerMsg: 0,
		MaxInflightMsgNum: 256,
		LastAppliedIndex:  0,
	})

	msg := raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		To:   1,
		From: 1,
	}
	rnd.sendToMailbox(msg)
	rnd.Step(msg)

	if rnd.state != raftpb.NODE_STATE_CANDIDATE {
		t.Fatalf("node state expected %q, got %q", raftpb.NODE_STATE_CANDIDATE, rnd.state)
	}
	if rnd.term != 1 {
		t.Fatalf("term expected 1, got %d", rnd.term)
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

func Test_raft_leader_election_leaderCheckQuorum_candidate(t *testing.T) {
	rnd1 := newRaftNode(&Config{
		ID:                      1,
		allPeerIDs:              []uint64{1, 2, 3},
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		LeaderCheckQuorum:       true,
		StorageStable:           NewStorageStableInMemory(),
		MaxEntryNumPerMsg:       0,
		MaxInflightMsgNum:       256,
		LastAppliedIndex:        0,
	})
	rnd2 := newRaftNode(&Config{
		ID:                      2,
		allPeerIDs:              []uint64{1, 2, 3},
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		LeaderCheckQuorum:       true,
		StorageStable:           NewStorageStableInMemory(),
		MaxEntryNumPerMsg:       0,
		MaxInflightMsgNum:       256,
		LastAppliedIndex:        0,
	})
	rnd3 := newRaftNode(&Config{
		ID:                      3,
		allPeerIDs:              []uint64{1, 2, 3},
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		LeaderCheckQuorum:       true,
		StorageStable:           NewStorageStableInMemory(),
		MaxEntryNumPerMsg:       0,
		MaxInflightMsgNum:       256,
		LastAppliedIndex:        0,
	})

	fn := newFakeNetwork(rnd1, rnd2, rnd3)

	// 1. leaderCheckQuorum is true
	// AND
	// 2. election time out hasn't passed yet
	//
	// THEN leader last confirmed its leadership, guaranteed to have been
	// in contact with quorum within the election timeout, so it will ignore
	// the vote request from candidate.

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
func Test_raft_leader_election_leaderCheckQuorum_leader(t *testing.T) {
	rnd1 := newRaftNode(&Config{
		ID:                      1,
		allPeerIDs:              []uint64{1, 2, 3},
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		LeaderCheckQuorum:       true,
		StorageStable:           NewStorageStableInMemory(),
		MaxEntryNumPerMsg:       0,
		MaxInflightMsgNum:       256,
		LastAppliedIndex:        0,
	})
	rnd2 := newRaftNode(&Config{
		ID:                      2,
		allPeerIDs:              []uint64{1, 2, 3},
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		LeaderCheckQuorum:       true,
		StorageStable:           NewStorageStableInMemory(),
		MaxEntryNumPerMsg:       0,
		MaxInflightMsgNum:       256,
		LastAppliedIndex:        0,
	})
	rnd3 := newRaftNode(&Config{
		ID:                      3,
		allPeerIDs:              []uint64{1, 2, 3},
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		LeaderCheckQuorum:       true,
		StorageStable:           NewStorageStableInMemory(),
		MaxEntryNumPerMsg:       0,
		MaxInflightMsgNum:       256,
		LastAppliedIndex:        0,
	})

	fn := newFakeNetwork(rnd1, rnd2, rnd3)

	// time out rnd2, so that rnd2 now does not ignore the vote request
	// and becomes the follower. And rnd1 will start campaign, and rnd2
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

	if rnd1.state != raftpb.NODE_STATE_LEADER {
		t.Fatalf("rnd1 state expected %q, got %q", raftpb.NODE_STATE_LEADER, rnd1.state)
	}
	if rnd2.state != raftpb.NODE_STATE_FOLLOWER {
		t.Fatalf("rnd2 state expected %q, got %q", raftpb.NODE_STATE_FOLLOWER, rnd2.state)
	}
	if rnd3.state != raftpb.NODE_STATE_FOLLOWER {
		t.Fatalf("rnd3 state expected %q, got %q", raftpb.NODE_STATE_FOLLOWER, rnd3.state)
	}

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

	if rnd1.state != raftpb.NODE_STATE_FOLLOWER {
		t.Fatalf("rnd1 state expected %q, got %q", raftpb.NODE_STATE_FOLLOWER, rnd1.state)
	}
	if rnd2.state != raftpb.NODE_STATE_FOLLOWER {
		t.Fatalf("rnd2 state expected %q, got %q", raftpb.NODE_STATE_FOLLOWER, rnd2.state)
	}
	if rnd3.state != raftpb.NODE_STATE_LEADER {
		t.Fatalf("rnd3 state expected %q, got %q", raftpb.NODE_STATE_LEADER, rnd3.state)
	}
}

// (etcd raft.TestLeaderSupersedingWithCheckQuorum)
func Test_raft_leader_election_leaderCheckQuorum_leader_ignore_vote(t *testing.T) {
	rnd1 := newRaftNode(&Config{
		ID:                      1,
		allPeerIDs:              []uint64{1, 2, 3},
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		LeaderCheckQuorum:       true,
		StorageStable:           NewStorageStableInMemory(),
		MaxEntryNumPerMsg:       0,
		MaxInflightMsgNum:       256,
		LastAppliedIndex:        0,
	})
	rnd2 := newRaftNode(&Config{
		ID:                      2,
		allPeerIDs:              []uint64{1, 2, 3},
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		LeaderCheckQuorum:       true,
		StorageStable:           NewStorageStableInMemory(),
		MaxEntryNumPerMsg:       0,
		MaxInflightMsgNum:       256,
		LastAppliedIndex:        0,
	})
	rnd3 := newRaftNode(&Config{
		ID:                      3,
		allPeerIDs:              []uint64{1, 2, 3},
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		LeaderCheckQuorum:       true,
		StorageStable:           NewStorageStableInMemory(),
		MaxEntryNumPerMsg:       0,
		MaxInflightMsgNum:       256,
		LastAppliedIndex:        0,
	})

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
	if rnd1.state != raftpb.NODE_STATE_LEADER {
		t.Fatalf("rnd1 state expected %q, got %q", raftpb.NODE_STATE_LEADER, rnd1.state)
	}
	if rnd2.state != raftpb.NODE_STATE_FOLLOWER {
		t.Fatalf("rnd2 state expected %q, got %q", raftpb.NODE_STATE_FOLLOWER, rnd2.state)
	}
	if rnd3.state != raftpb.NODE_STATE_FOLLOWER {
		t.Fatalf("rnd3 state expected %q, got %q", raftpb.NODE_STATE_FOLLOWER, rnd3.state)
	}

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
