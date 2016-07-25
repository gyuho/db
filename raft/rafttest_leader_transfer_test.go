package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestLeaderTransferToUpToDateNode)
func Test_raft_leader_transfer_up_to_date(t *testing.T) {
	fn := newFakeNetwork(nil, nil, nil)

	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})

	rndLeader1 := fn.allStateMachines[1].(*raftNode)
	rndLeader1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rndLeader1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rndLeader1.leaderID)
	}

	// transfer leader from 1 to 2
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_LEADER_TRANSFER,
		From: 2,
		To:   1,
	})

	// now node 2 is the new leader
	rndLeader1.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	if rndLeader1.leaderID != 2 {
		t.Fatalf("leaderID expected 2, got %d", rndLeader1.leaderID)
	}
	if rndLeader1.leaderTransfereeID != 0 {
		t.Fatalf("after leader transfer, leaderTransfereeID expected 0, got %d", rndLeader1.leaderTransfereeID)
	}

	// write some logs
	fn.stepFirstMessage(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		From:    1,
		To:      1,
		Entries: []raftpb.Entry{{Data: []byte("testdata")}},
	})

	// transfer leadership back from 2 to 1
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_LEADER_TRANSFER,
		From: 1,
		To:   2,
	})
	rndLeader1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rndLeader1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rndLeader1.leaderID)
	}
	if rndLeader1.leaderTransfereeID != 0 {
		t.Fatalf("after leader transfer, leaderTransfereeID expected 0, got %d", rndLeader1.leaderTransfereeID)
	}
}

// (etcd raft.TestLeaderTransferWithCheckQuorum)
func Test_raft_leader_transfer_with_check_quorum(t *testing.T) {
	fn := newFakeNetwork(nil, nil, nil)
	for i := 1; i <= 3; i++ {
		rnd := fn.allStateMachines[uint64(i)].(*raftNode)
		rnd.checkQuorum = true
	}

	// rnd.checkQuorum = true
	// so we need to election-timeout one node to vote for 1
	rnd2 := fn.allStateMachines[2].(*raftNode)
	for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
		rnd2.tickFunc()
	}

	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})

	rndLeader1 := fn.allStateMachines[1].(*raftNode)
	rndLeader1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rndLeader1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rndLeader1.leaderID)
	}

	// transfer leader from 1 to 2
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_LEADER_TRANSFER,
		From: 2,
		To:   1,
	})

	// now node 2 is the new leader
	rndLeader1.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	if rndLeader1.leaderID != 2 {
		t.Fatalf("leaderID expected 2, got %d", rndLeader1.leaderID)
	}
	if rndLeader1.leaderTransfereeID != 0 {
		t.Fatalf("after leader transfer, leaderTransfereeID expected 0, got %d", rndLeader1.leaderTransfereeID)
	}

	// write some logs
	fn.stepFirstMessage(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		From:    1,
		To:      1,
		Entries: []raftpb.Entry{{Data: []byte("testdata")}},
	})

	// transfer leadership back from 2 to 1
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_LEADER_TRANSFER,
		From: 1,
		To:   2,
	})
	rndLeader1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rndLeader1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rndLeader1.leaderID)
	}
	if rndLeader1.leaderTransfereeID != 0 {
		t.Fatalf("after leader transfer, leaderTransfereeID expected 0, got %d", rndLeader1.leaderTransfereeID)
	}
}

// (etcd raft.TestLeaderTransferToSlowFollower)

// (etcd raft.TestLeaderTransferAfterSnapshot)

// (etcd raft.TestLeaderTransferToSelf)

// (etcd raft.TestLeaderTransferToNonExistingNode)

// (etcd raft.TestLeaderTransferTimeout)

// (etcd raft.TestLeaderTransferIgnoreProposal)

// (etcd raft.TestLeaderTransferReceiveHigherTermVote)

// (etcd raft.TestLeaderTransferRemoveNode)

// (etcd raft.TestLeaderTransferBack)

// (etcd raft.TestLeaderTransferSecondTransferToAnotherNode)

// (etcd raft.TestLeaderTransferSecondTransferToSameNode)
