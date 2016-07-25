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
func Test_raft_recover_pending_config(t *testing.T) {
	tests := []struct {
		entryTypeToAppend   raftpb.ENTRY_TYPE
		wPendingConfigExist bool
	}{
		{raftpb.ENTRY_TYPE_NORMAL, false},
		{raftpb.ENTRY_TYPE_CONFIG_CHANGE, true},
	}

	for i, tt := range tests {
		rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())

		rnd.leaderAppendEntriesToLeader(raftpb.Entry{Type: tt.entryTypeToAppend})

		rnd.becomeCandidate()
		rnd.becomeLeader()

		// (X)
		// rnd.leaderAppendEntriesToLeader(raftpb.Entry{Type: tt.entryTypeToAppend})

		if rnd.pendingConfigExist != tt.wPendingConfigExist {
			t.Fatalf("#%d: pendingConfigExist expected %v, got %v", i, tt.wPendingConfigExist, rnd.pendingConfigExist)
		}
	}
}

// (etcd raft.TestRecoverDoublePendingConfig)
func Test_raft_recover_double_pending_config(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Logf("panic with %v", err)
		} else {
			t.Fatal("expect panic, but nothing happens")
		}
	}()

	rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())
	rnd.leaderAppendEntriesToLeader(raftpb.Entry{Type: raftpb.ENTRY_TYPE_CONFIG_CHANGE})
	rnd.leaderAppendEntriesToLeader(raftpb.Entry{Type: raftpb.ENTRY_TYPE_CONFIG_CHANGE})
	rnd.becomeCandidate()
	rnd.becomeLeader()
}

// (etcd raft.TestAddNode)
func Test_raft_add_node(t *testing.T) {

}

// (etcd raft.TestRemoveNode)
func Test_raft_remove_node(t *testing.T) {

}

// (etcd raft.TestCommitAfterRemoveNode)
func Test_raft_commit_after_remove_node(t *testing.T) {

}

// (etcd raft.TestNodeProposeConfig)
func Test_raft_node_propose_config(t *testing.T) {

}
