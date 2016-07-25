package raft

import (
	"bytes"
	"context"
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
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())
	rnd.pendingConfigExist = true

	rnd.addNode(2)

	if rnd.pendingConfigExist {
		t.Fatalf("pendingConfigExist expected false, got %v", rnd.pendingConfigExist)
	}

	nodeIDs := rnd.allNodeIDs()
	wNodeIDs := []uint64{1, 2}
	if !reflect.DeepEqual(nodeIDs, wNodeIDs) {
		t.Fatalf("node ids expected %+v, got %+v", wNodeIDs, nodeIDs)
	}
}

// (etcd raft.TestRemoveNode)
func Test_raft_delete_node(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())
	rnd.pendingConfigExist = true

	rnd.deleteNode(2)

	if rnd.pendingConfigExist {
		t.Fatalf("pendingConfigExist expected false, got %v", rnd.pendingConfigExist)
	}

	nodeIDs := rnd.allNodeIDs()
	wNodeIDs := []uint64{1}
	if !reflect.DeepEqual(nodeIDs, wNodeIDs) {
		t.Fatalf("node ids expected %+v, got %+v", wNodeIDs, nodeIDs)
	}

	rnd.deleteNode(1)

	nodeIDs = rnd.allNodeIDs()
	wNodeIDs = []uint64{}
	if !reflect.DeepEqual(nodeIDs, wNodeIDs) {
		t.Fatalf("node ids expected %+v, got %+v", wNodeIDs, nodeIDs)
	}
}

// (etcd raft.TestCommitAfterRemoveNode)
func Test_raft_commit_after_delete_node(t *testing.T) {
	st := NewStorageStableInMemory()
	rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, st)
	rnd.becomeCandidate()
	rnd.becomeLeader()

	configChange := raftpb.ConfigChange{
		Type:   raftpb.CONFIG_CHANGE_TYPE_REMOVE_NODE,
		NodeID: 2,
	}
	configChangeData, err := configChange.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	rnd.Step(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		Entries: []raftpb.Entry{{Type: raftpb.ENTRY_TYPE_CONFIG_CHANGE, Data: configChangeData}},
	})

	nextEnts := persistALlUnstableAndApplyNextEntries(rnd, st)
	if len(nextEnts) > 0 {
		t.Fatalf("unexpected committed entries %+v", nextEnts)
	}
	lastIndex := rnd.storageRaftLog.lastIndex()

	// while config change is pending, make another proposal
	rnd.Step(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		Entries: []raftpb.Entry{{Type: raftpb.ENTRY_TYPE_NORMAL, Data: []byte("testdata")}},
	})

	// node 2 acknowledges the config change and commits it
	rnd.Step(raftpb.Message{
		Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND,
		From:     2,
		LogIndex: lastIndex,
	})

	nextEnts = persistALlUnstableAndApplyNextEntries(rnd, st)
	if len(nextEnts) != 2 {
		t.Fatalf("len(nextEnts) expected 2, got %v", len(nextEnts))
	}
	if nextEnts[0].Type != raftpb.ENTRY_TYPE_NORMAL || nextEnts[0].Data != nil {
		t.Fatalf("first entry expected empty, got %+v", nextEnts[0])
	}
	if nextEnts[1].Type != raftpb.ENTRY_TYPE_CONFIG_CHANGE {
		t.Fatalf("nextEnts[1] expected raftpb.ENTRY_TYPE_CONFIG_CHANGE, got %+v", nextEnts[1])
	}
}

// (etcd raft.TestNodeProposeConfig)
func Test_raft_node_propose_config(t *testing.T) {
	var msgs []raftpb.Message
	stepFuncAppend := func(rnd *raftNode, msg raftpb.Message) {
		msgs = append(msgs, msg)
	}

	nd := newNode()
	st := NewStorageStableInMemory()
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, st)

	go nd.runWithRaftNode(rnd)

	nd.Campaign(context.TODO())

	for {
		rd := <-nd.Ready()
		st.Append(rd.EntriesToAppend...)

		// until this raft node becomes leader
		if rd.SoftState.LeaderID == rnd.id {
			rnd.stepFunc = stepFuncAppend
			nd.Advance()
			break
		}

		nd.Advance()
	}

	configChange := raftpb.ConfigChange{
		Type:   raftpb.CONFIG_CHANGE_TYPE_ADD_NODE,
		NodeID: 1,
	}
	nd.ProposeConfigChange(context.TODO(), configChange)
	nd.Stop()

	if len(msgs) != 1 {
		t.Fatalf("len(msgs) expected 1, got %d", len(msgs))
	}
	if msgs[0].Type != raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER {
		t.Fatalf("message type expected %q, got %q", raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, msgs[0].Type)
	}

	configChangeData, err := configChange.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msgs[0].Entries[0].Data, configChangeData) {
		t.Fatalf("data expected %s, got %s", configChangeData, msgs[0].Entries[0].Data)
	}
}
