package raft

import (
	"math"
	"reflect"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestStepConfig)
func Test_raft_config_change_Step(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())
	rnd.becomeCandidate()
	rnd.becomeLeader()

	lastIndex1 := rnd.storageRaftLog.lastIndex()

	rnd.Step(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		From:    1,
		To:      1,
		Entries: []raftpb.Entry{{Type: raftpb.ENTRY_TYPE_CONFIG_CHANGE}},
	})

	lastIndex2 := rnd.storageRaftLog.lastIndex()
	if lastIndex2 != lastIndex1+1 { // entry added, so last index + 1
		t.Fatalf("last index expected %d, got %d", lastIndex1+1, lastIndex2)
	}
	if !rnd.pendingConfigExist {
		t.Fatalf("pendingConfigExist expected true, got %v", rnd.pendingConfigExist)
	}
}

// (etcd raft.TestStepIgnoreConfig)
func Test_raft_config_change_Step_ignore(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())
	rnd.becomeCandidate()
	rnd.becomeLeader()

	rnd.Step(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		From:    1,
		To:      1,
		Entries: []raftpb.Entry{{Type: raftpb.ENTRY_TYPE_CONFIG_CHANGE}},
	})
	lastIndex1 := rnd.storageRaftLog.lastIndex()
	pendingConfigExist1 := rnd.pendingConfigExist

	rnd.Step(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		From:    1,
		To:      1,
		Entries: []raftpb.Entry{{Type: raftpb.ENTRY_TYPE_CONFIG_CHANGE}},
	})

	lastIndex2 := rnd.storageRaftLog.lastIndex()
	pendingConfigExist2 := rnd.pendingConfigExist

	if lastIndex2 != lastIndex1+1 { // entry added, so last index + 1
		t.Fatalf("last index expected %d, got %d", lastIndex1+1, lastIndex2)
	}
	if !rnd.pendingConfigExist {
		t.Fatalf("pendingConfigExist expected true, got %v", rnd.pendingConfigExist)
	}
	if pendingConfigExist1 != pendingConfigExist2 {
		t.Fatalf("pendingConfigExist1 %v != pendingConfigExist2 %v", pendingConfigExist1, pendingConfigExist2)
	}

	ents, err := rnd.storageRaftLog.entries(lastIndex2, math.MaxUint64)
	if err != nil {
		t.Fatal(err)
	}
	wents := []raftpb.Entry{{Type: raftpb.ENTRY_TYPE_NORMAL, Index: 3, Term: 1}}
	if !reflect.DeepEqual(ents, wents) {
		t.Fatalf("entries expected %+v, got %+v", wents, ents)
	}
}

// (etcd raft.TestRecoverPendingConfig)

// (etcd raft.TestRecoverDoublePendingConfig)

// (etcd raft.TestAddNode)

// (etcd raft.TestRemoveNode)

// (etcd raft.TestCommitAfterRemoveNode)

// (etcd raft.TestNodeProposeConfig)
