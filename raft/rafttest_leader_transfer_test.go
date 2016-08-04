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

	rnd1 := fn.allStateMachines[1].(*raftNode)
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}

	// transfer leader from 1 to 2
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 2,
		To:   1,
	})

	// now node 2 is the new leader
	rnd1.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	if rnd1.leaderID != 2 {
		t.Fatalf("leaderID expected 2, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rnd1.leaderTransfereeID)
	}

	// write some logs
	fn.stepFirstMessage(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		From:    1,
		To:      1,
		Entries: []raftpb.Entry{{}},
	})

	// transfer leadership back from 2 to 1
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 1,
		To:   2,
	})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rnd1.leaderTransfereeID)
	}
}

// (etcd raft.TestLeaderTransferToSelf)
func Test_raft_leader_transfer_to_self(t *testing.T) {
	fn := newFakeNetwork(nil, nil, nil)

	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})

	rnd1 := fn.allStateMachines[1].(*raftNode)

	// transfer leader from 1 to 1
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 1,
		To:   1,
	})

	// now 1 is the leader
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rnd1.leaderTransfereeID)
	}
}

// (etcd raft.TestLeaderTransferBack)
func Test_raft_leader_transfer_back(t *testing.T) {
	fn := newFakeNetwork(nil, nil, nil)

	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1 := fn.allStateMachines[1].(*raftNode)
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)

	fn.isolate(3)

	// transfer leader from 1 to 3
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 3,
		To:   1,
	})

	// 1 is still the leader
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 3 {
		t.Fatalf("leaderTransfereeID expected 3, got %d", rnd1.leaderTransfereeID)
	}

	// transfer leader from 1 to 1
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 1,
		To:   1,
	})

	// 1 is the leader
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rnd1.leaderTransfereeID)
	}
}

// (etcd raft.TestLeaderTransferWithCheckQuorum)
func Test_raft_leader_transfer_with_check_quorum(t *testing.T) {
	fn := newFakeNetwork(nil, nil, nil)
	for i := 1; i <= 3; i++ {
		rnd := fn.allStateMachines[uint64(i)].(*raftNode)
		rnd.checkQuorum = true
		rnd.setRandomizedElectionTimeoutTickNum(rnd.electionTimeoutTickNum + i)
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

	rnd1 := fn.allStateMachines[1].(*raftNode)
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}

	// transfer leader from 1 to 2
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 2,
		To:   1,
	})

	// now node 2 is the new leader
	rnd1.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	if rnd1.leaderID != 2 {
		t.Fatalf("leaderID expected 2, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rnd1.leaderTransfereeID)
	}

	// write some logs
	fn.stepFirstMessage(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		From:    1,
		To:      1,
		Entries: []raftpb.Entry{{}},
	})

	// transfer leadership back from 2 to 1
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 1,
		To:   2,
	})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rnd1.leaderTransfereeID)
	}
}

// (etcd raft.TestLeaderTransferToSlowFollower)
func Test_raft_leader_transfer_to_slow_follower(t *testing.T) {
	fn := newFakeNetwork(nil, nil, nil)

	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1 := fn.allStateMachines[1].(*raftNode)
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)

	fn.isolate(3)

	// write some logs
	fn.stepFirstMessage(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		From:    1,
		To:      1,
		Entries: []raftpb.Entry{{}},
	})

	fn.recoverAll()

	if rnd1.allProgresses[2].MatchIndex != 2 {
		t.Fatalf("rnd1.allProgresses[2].MatchIndex expected 2, got %d", rnd1.allProgresses[2].MatchIndex)
	}
	if rnd1.allProgresses[3].MatchIndex != 1 { // 3 should be fallen behind
		t.Fatalf("rnd1.allProgresses[3].MatchIndex expected 1, got %d", rnd1.allProgresses[3].MatchIndex)
	}

	// transfer leader from 1 to 3
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 3,
		To:   1,
	})
	// now node 3 is the new leader
	rndLeader3 := fn.allStateMachines[3].(*raftNode)

	rnd1.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	if rnd1.leaderID != 3 {
		t.Fatalf("leaderID expected 3, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rnd1.leaderTransfereeID)
	}

	rndLeader3.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rndLeader3.leaderID != 3 {
		t.Fatalf("leaderID expected 3, got %d", rndLeader3.leaderID)
	}
	if rndLeader3.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rndLeader3.leaderTransfereeID)
	}
}

// (etcd raft.TestLeaderTransferAfterSnapshot)
func Test_raft_leader_transfer_after_snapshot(t *testing.T) {
	fn := newFakeNetwork(nil, nil, nil)

	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1 := fn.allStateMachines[1].(*raftNode)
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)

	fn.isolate(3)

	// write some logs
	fn.stepFirstMessage(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		From:    1,
		To:      1,
		Entries: []raftpb.Entry{{}},
	})
	persistALlUnstableAndApplyNextEntries(rnd1, fn.allStableStorageInMemory[1])

	fn.allStableStorageInMemory[1].CreateSnapshot(
		rnd1.storageRaftLog.appliedIndex,
		&raftpb.ConfigState{IDs: rnd1.allNodeIDs()},
		nil,
	)
	fn.allStableStorageInMemory[1].Compact(rnd1.storageRaftLog.appliedIndex)

	fn.recoverAll()

	if rnd1.allProgresses[2].MatchIndex != 2 {
		t.Fatalf("rnd1.allProgresses[2].MatchIndex expected 1, got %d", rnd1.allProgresses[2].MatchIndex)
	}
	if rnd1.allProgresses[3].MatchIndex != 1 { // 3 should be fallen behind
		t.Fatalf("rnd1.allProgresses[3].MatchIndex expected 1, got %d", rnd1.allProgresses[3].MatchIndex)
	}

	// transfer leader from 1 to 3, when node 3 is lack of snapshot
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 3,
		To:   1,
	})

	// 1 is now follower
	rnd1.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

	// 3 is the new leader
	rndLeader3 := fn.allStateMachines[3].(*raftNode)
	rndLeader3.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rndLeader3.leaderID != 3 {
		t.Fatalf("leaderID expected 3, got %d", rndLeader3.leaderID)
	}
	if rndLeader3.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rndLeader3.leaderTransfereeID)
	}

	// heartbeat to be ignored
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT,
		From: 3,
		To:   1,
	})

	// 1 is still follower
	rnd1.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

	// 3 is still leader
	rndLeader3.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rndLeader3.leaderID != 3 {
		t.Fatalf("leaderID expected 3, got %d", rndLeader3.leaderID)
	}
	if rndLeader3.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rndLeader3.leaderTransfereeID)
	}
}

// (etcd raft.TestLeaderTransferToNonExistingNode)
func Test_raft_leader_transfer_to_non_existing_node(t *testing.T) {
	fn := newFakeNetwork(nil, nil, nil)

	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1 := fn.allStateMachines[1].(*raftNode)
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)

	// transfer leader from 1 to 3
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 4,
		To:   1,
	})

	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rnd1.leaderTransfereeID)
	}
}

// (etcd raft.TestLeaderTransferTimeout)
func Test_raft_leader_transfer_timeout(t *testing.T) {
	fn := newFakeNetwork(nil, nil, nil)

	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1 := fn.allStateMachines[1].(*raftNode)
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)

	fn.isolate(3)

	// transfer leader from 1 to 3
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 3,
		To:   1,
	})

	// 1 is still the leader
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 3 {
		t.Fatalf("leaderTransfereeID expected 3, got %d", rnd1.leaderTransfereeID)
	}
	for i := 0; i < rnd1.heartbeatTimeoutTickNum; i++ {
		rnd1.tickFunc()
	}
	if rnd1.leaderTransfereeID != 3 {
		t.Fatalf("leaderTransfereeID expected 3, got %d", rnd1.leaderTransfereeID)
	}

	for i := 0; i < rnd1.electionTimeoutTickNum-rnd1.heartbeatTimeoutTickNum; i++ {
		rnd1.tickFunc()
	}

	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rnd1.leaderTransfereeID)
	}
}

// (etcd raft.TestLeaderTransferIgnoreProposal)
func Test_raft_leader_transfer_ignore_proposal(t *testing.T) {
	fn := newFakeNetwork(nil, nil, nil)

	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1 := fn.allStateMachines[1].(*raftNode)
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)

	fn.isolate(3)

	// transfer leader from 1 to 3
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 3,
		To:   1,
	})

	// 1 is still the leader
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 3 {
		t.Fatalf("leaderTransfereeID expected 3, got %d", rnd1.leaderTransfereeID)
	}

	// write some logs
	fn.stepFirstMessage(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		From:    1,
		To:      1,
		Entries: []raftpb.Entry{{}},
	})
	if rnd1.allProgresses[1].MatchIndex != 1 {
		t.Fatalf("rnd1.allProgresses[1].MatchIndex expected 1, got %d", rnd1.allProgresses[1].MatchIndex)
	}
	if rnd1.allProgresses[2].MatchIndex != 1 {
		t.Fatalf("rnd1.allProgresses[2].MatchIndex expected 1, got %d", rnd1.allProgresses[2].MatchIndex)
	}
	if rnd1.allProgresses[3].MatchIndex != 1 {
		t.Fatalf("rnd1.allProgresses[3].MatchIndex expected 1, got %d", rnd1.allProgresses[3].MatchIndex)
	}
}

// (etcd raft.TestLeaderTransferReceiveHigherTermVote)
func Test_raft_leader_transfer_receive_higher_term_vote(t *testing.T) {
	fn := newFakeNetwork(nil, nil, nil)

	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1 := fn.allStateMachines[1].(*raftNode)
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)

	fn.isolate(3)

	// transfer leader from 1 to 3
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 3,
		To:   1,
	})

	// 1 is still the leader
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 3 {
		t.Fatalf("leaderTransfereeID expected 3, got %d", rnd1.leaderTransfereeID)
	}

	if rnd1.currentTerm != 1 {
		t.Fatalf("current term expected 1, got %d", rnd1.currentTerm)
	}

	// trigger campaign in node 2 with higher term
	fn.stepFirstMessage(raftpb.Message{
		Type:              raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From:              2,
		To:                2,
		SenderCurrentTerm: 2,
		LogIndex:          1,
	})

	// now node 1 reverts back to follower
	rnd1.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	if rnd1.leaderID != 2 {
		t.Fatalf("leaderID expected 2, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rnd1.leaderTransfereeID)
	}
}

// (etcd raft.TestLeaderTransferRemoveNode)
func Test_raft_leader_transfer_delete_node(t *testing.T) {
	fn := newFakeNetwork(nil, nil, nil)

	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1 := fn.allStateMachines[1].(*raftNode)
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)

	// ignore election-timeout message
	fn.ignoreMessageType(raftpb.MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT)

	// transfer leader from 1 to 3
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 3,
		To:   1,
	})

	// 1 is still the leader
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 3 {
		t.Fatalf("leaderTransfereeID expected 3, got %d", rnd1.leaderTransfereeID)
	}

	rnd1.deleteNode(3)

	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rnd1.leaderTransfereeID)
	}
}

// (etcd raft.TestLeaderTransferSecondTransferToAnotherNode)
func Test_raft_leader_transfer_second_transfer(t *testing.T) {
	fn := newFakeNetwork(nil, nil, nil)

	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1 := fn.allStateMachines[1].(*raftNode)
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)

	fn.isolate(3)

	// transfer leader from 1 to 3
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 3,
		To:   1,
	})

	// 1 is still the leader
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 3 {
		t.Fatalf("leaderTransfereeID expected 3, got %d", rnd1.leaderTransfereeID)
	}

	// transfer leader from 1 to 2
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 2,
		To:   1,
	})

	// 2 is the new leader
	rnd1.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd2 := fn.allStateMachines[2].(*raftNode)
	rnd2.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 2 {
		t.Fatalf("leaderID expected 2, got %d", rnd1.leaderID)
	}
	if rnd2.leaderID != 2 {
		t.Fatalf("leaderID expected 2, got %d", rnd2.leaderID)
	}
	if rnd1.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rnd1.leaderTransfereeID)
	}
}

// (etcd raft.TestLeaderTransferSecondTransferToSameNode)
func Test_raft_leader_transfer_second_transfer_to_same_node(t *testing.T) {
	fn := newFakeNetwork(nil, nil, nil)

	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1 := fn.allStateMachines[1].(*raftNode)
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)

	fn.isolate(3)

	// transfer leader from 1 to 3
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 3,
		To:   1,
	})

	// 1 is still the leader
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 3 {
		t.Fatalf("leaderTransfereeID expected 3, got %d", rnd1.leaderTransfereeID)
	}

	for i := 0; i < rnd1.heartbeatTimeoutTickNum; i++ {
		rnd1.tickFunc()
	}

	// again, transfer leader from 1 to 3
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_TRANSFER_LEADER,
		From: 3,
		To:   1,
	})

	for i := 0; i < rnd1.electionTimeoutTickNum-rnd1.heartbeatTimeoutTickNum; i++ {
		rnd1.tickFunc()
	}

	// 1 is still the leader
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leaderID expected 1, got %d", rnd1.leaderID)
	}
	if rnd1.leaderTransfereeID != 0 {
		t.Fatalf("leaderTransfereeID expected 0, got %d", rnd1.leaderTransfereeID)
	}
	//
	// tickFuncLeaderHeartbeatTimeout
	// if rnd.electionTimeoutElapsedTickNum >= rnd.electionTimeoutTickNum {
	// then stop leader transfer
}
