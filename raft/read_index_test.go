package raft

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestReadOnlyOptionSafe)
func Test_raft_read_index_ReadOnlySafe_without_check_quorum(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd3 := newTestRaftNode(3, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	fn := newFakeNetwork(rnd1, rnd2, rnd3)
	rnd2.setRandomizedElectionTimeoutTickNum(rnd2.electionTimeoutTickNum + 1)

	// trigger election in rnd2
	for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
		rnd2.tickFunc()
	}

	// trigger campaign in rnd1
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)

	tests := []struct {
		rnd                *raftNode
		proposalsToLeaderN int
		wReadIndex         uint64

		requestCtxToSend []byte
	}{
		{rnd2, 10, 11, []byte("ctx1")},
		{rnd3, 10, 21, []byte("ctx2")},
		{rnd2, 10, 31, []byte("ctx3")},
		{rnd3, 10, 41, []byte("ctx4")},
	}

	for i, tt := range tests {
		for j := 0; j < tt.proposalsToLeaderN; j++ {
			fn.stepFirstMessage(raftpb.Message{
				Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
				From:    1,
				To:      1,
				Entries: []raftpb.Entry{{}},
			})
		}

		fn.stepFirstMessage(raftpb.Message{
			Type:    raftpb.MESSAGE_TYPE_TRIGGER_READ_INDEX,
			From:    tt.rnd.id,
			To:      tt.rnd.id,
			Entries: []raftpb.Entry{{Data: tt.requestCtxToSend}},
		})

		rs := tt.rnd.readStates[0]
		if rs.Index != tt.wReadIndex {
			t.Fatalf("#%d: read index expected %d, got %d", i, tt.wReadIndex, rs.Index)
		}
		if !bytes.Equal(rs.RequestCtx, tt.requestCtxToSend) {
			t.Fatalf("#%d: request ctx expected %s, got %s", i, tt.requestCtxToSend, rs.RequestCtx)
		}
		tt.rnd.readStates = nil
	}
}

// (etcd raft.TestReadOnlyOptionLease)
func Test_raft_read_index_ReadOnlyLeaseBased_with_check_quorum(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd3 := newTestRaftNode(3, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd1.readOnly.option = ReadOnlyLeaseBased
	rnd2.readOnly.option = ReadOnlyLeaseBased
	rnd3.readOnly.option = ReadOnlyLeaseBased

	rnd1.checkQuorum = true
	rnd2.checkQuorum = true
	rnd3.checkQuorum = true

	fn := newFakeNetwork(rnd1, rnd2, rnd3)
	rnd2.setRandomizedElectionTimeoutTickNum(rnd2.electionTimeoutTickNum + 1)

	// trigger election in rnd2
	for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
		rnd2.tickFunc()
	}

	// trigger campaign in rnd1
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)

	tests := []struct {
		rnd                *raftNode
		proposalsToLeaderN int
		wReadIndex         uint64

		requestCtxToSend []byte
	}{
		{rnd2, 10, 11, []byte("ctx1")},
		{rnd3, 10, 21, []byte("ctx2")},
		{rnd2, 10, 31, []byte("ctx3")},
		{rnd3, 10, 41, []byte("ctx4")},
	}

	for i, tt := range tests {
		for j := 0; j < tt.proposalsToLeaderN; j++ {
			fn.stepFirstMessage(raftpb.Message{
				Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
				From:    1,
				To:      1,
				Entries: []raftpb.Entry{{}},
			})
		}

		fn.stepFirstMessage(raftpb.Message{
			Type:    raftpb.MESSAGE_TYPE_TRIGGER_READ_INDEX,
			From:    tt.rnd.id,
			To:      tt.rnd.id,
			Entries: []raftpb.Entry{{Data: tt.requestCtxToSend}},
		})

		rs := tt.rnd.readStates[0]
		if rs.Index != tt.wReadIndex {
			t.Fatalf("#%d: read index expected %d, got %d", i, tt.wReadIndex, rs.Index)
		}
		if !bytes.Equal(rs.RequestCtx, tt.requestCtxToSend) {
			t.Fatalf("#%d: request ctx expected %s, got %s", i, tt.requestCtxToSend, rs.RequestCtx)
		}
		tt.rnd.readStates = nil
	}
}

// (etcd raft.TestReadOnlyOptionLeaseWithoutCheckQuorum)
func Test_raft_read_index_ReadOnlyLeaseBased_without_check_quorum(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd3 := newTestRaftNode(3, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd1.readOnly.option = ReadOnlyLeaseBased
	rnd2.readOnly.option = ReadOnlyLeaseBased
	rnd3.readOnly.option = ReadOnlyLeaseBased

	fn := newFakeNetwork(rnd1, rnd2, rnd3)

	// trigger campaign in rnd1
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 1,
		To:   1,
	})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)

	requestCtxToSend := []byte("testcontext")
	fn.stepFirstMessage(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_TRIGGER_READ_INDEX,
		From:    2,
		To:      2,
		Entries: []raftpb.Entry{{Data: requestCtxToSend}},
	})

	rs := rnd2.readStates[0]
	if rs.Index != uint64(0) {
		t.Fatalf("readIndex expected %d, got %d", 0, rs.Index)
	}
	if !bytes.Equal(rs.RequestCtx, requestCtxToSend) {
		t.Fatalf("request ctx expected %s, got %s", requestCtxToSend, rs.RequestCtx)
	}
}

// (etcd raft.TestNodeReadIndex)
func Test_raft_node_read_index(t *testing.T) {
	var msgs []raftpb.Message
	stepFuncAppend := func(rnd *raftNode, msg raftpb.Message) {
		msgs = append(msgs, msg)
	}
	wrs := []ReadState{{Index: uint64(1), RequestCtx: []byte("testdata")}}

	nd := newNode()
	st := NewStorageStableInMemory()
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, st)
	rnd.readStates = wrs

	go nd.runWithRaftNode(rnd)

	nd.Campaign(context.TODO())

	for {
		rd := <-nd.Ready()
		if !reflect.DeepEqual(rd.ReadStates, wrs) {
			t.Fatalf("ReadStates expected %+v, got %+v", wrs, rd.ReadStates)
		}

		st.Append(rd.EntriesToAppend...)

		// until this raft node becomes leader
		if rd.SoftState.LeaderID == rnd.id {
			rnd.stepFunc = stepFuncAppend
			nd.Advance()
			break
		}

		nd.Advance()
	}

	wRequestCtx := []byte("testdata2")
	nd.ReadIndex(context.TODO(), wRequestCtx)
	nd.Stop()

	if len(msgs) != 1 {
		t.Fatalf("len(msgs) expected 1, got %d", len(msgs))
	}
	if msgs[0].Type != raftpb.MESSAGE_TYPE_TRIGGER_READ_INDEX {
		t.Fatalf("msgs[0].Type expected %q, got %q", raftpb.MESSAGE_TYPE_TRIGGER_READ_INDEX, msgs[0].Type)
	}
	if !bytes.Equal(msgs[0].Entries[0].Data, wRequestCtx) {
		t.Fatalf("data expected %s, got %s", wRequestCtx, msgs[0].Entries[0].Data)
	}
}
