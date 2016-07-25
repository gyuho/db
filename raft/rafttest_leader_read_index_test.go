package raft

import (
	"bytes"
	"context"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestReadIndexWithCheckQuorum)
func Test_raft_read_index_with_check_quorum(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd3 := newTestRaftNode(3, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd1.checkQuorum = true
	rnd2.checkQuorum = true
	rnd3.checkQuorum = true

	fn := newFakeNetwork(rnd1, rnd2, rnd3)

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
			Type:    raftpb.MESSAGE_TYPE_READ_INDEX,
			From:    tt.rnd.id,
			To:      tt.rnd.id,
			Entries: []raftpb.Entry{{Data: tt.requestCtxToSend}},
		})

		if tt.rnd.readState.Index != tt.wReadIndex {
			t.Fatalf("#%d: read index expected %d, got %d", i, tt.wReadIndex, tt.rnd.readState.Index)
		}

		if !bytes.Equal(tt.rnd.readState.RequestCtx, tt.requestCtxToSend) {
			t.Fatalf("#%d: request ctx expected %s, got %s", i, tt.requestCtxToSend, tt.rnd.readState.RequestCtx)
		}
	}
}

// (etcd raft.TestReadIndexWithoutCheckQuorum)
func Test_raft_read_index_without_check_quorum(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd3 := newTestRaftNode(3, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

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
		Type:    raftpb.MESSAGE_TYPE_READ_INDEX,
		From:    2,
		To:      2,
		Entries: []raftpb.Entry{{Data: requestCtxToSend}},
	})

	if rnd2.readState.Index != 0 {
		t.Fatalf("rnd2 readState.index expected 0, got %d", rnd2.readState.Index)
	}
	if !bytes.Equal(rnd2.readState.RequestCtx, requestCtxToSend) {
		t.Fatalf("request ctx expected %s, got %s", requestCtxToSend, rnd2.readState.RequestCtx)
	}
}

// (etcd raft.TestNodeReadIndex)
func Test_raft_node_read_index(t *testing.T) {
	var msgs []raftpb.Message
	stepFuncAppend := func(rnd *raftNode, msg raftpb.Message) {
		msgs = append(msgs, msg)
	}
	wReadIndex := uint64(1)
	wRequestCtx := []byte("testdata")

	nd := newNode()
	st := NewStorageStableInMemory()
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, st)
	rnd.readState.Index = wReadIndex
	rnd.readState.RequestCtx = wRequestCtx

	go nd.runWithRaftNode(rnd)

	nd.Campaign(context.TODO())

	for {
		rd := <-nd.Ready()
		if rd.ReadState.Index != wReadIndex {
			t.Fatalf("read index expected %d, got %d", wReadIndex, rd.ReadState.Index)
		}
		if !bytes.Equal(rd.ReadState.RequestCtx, wRequestCtx) {
			t.Fatalf("request ctx expected %s, got %s", wRequestCtx, rd.ReadState.RequestCtx)
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

	wRequestCtx = []byte("testdata2")
	nd.ReadIndex(context.TODO(), wRequestCtx)
	nd.Stop()

	if len(msgs) != 1 {
		t.Fatalf("len(msgs) expected 1, got %d", len(msgs))
	}
	if msgs[0].Type != raftpb.MESSAGE_TYPE_READ_INDEX {
		t.Fatalf("msgs[0].Type expected %q, got %q", raftpb.MESSAGE_TYPE_READ_INDEX, msgs[0].Type)
	}
	if !bytes.Equal(msgs[0].Entries[0].Data, wRequestCtx) {
		t.Fatalf("data expected %s, got %s", wRequestCtx, msgs[0].Entries[0].Data)
	}
}
