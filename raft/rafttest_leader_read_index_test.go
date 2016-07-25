package raft

import (
	"bytes"
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

}

// (etcd raft.TestNodeReadIndex)
func Test_raft_node_read_index(t *testing.T) {

}
