package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestMsgAppFlowControlFull)
func Test_raft_inflights_full(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())
	rnd.becomeCandidate()
	rnd.becomeLeader()

	pr2 := rnd.allProgresses[2]

	// force 2 progress to be replicate
	pr2.becomeReplicate()

	// fill in inflights window
	for i := 0; i < rnd.maxInflightMsgNum; i++ {
		rnd.Step(raftpb.Message{
			Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
			From:    1,
			To:      1,
			Entries: []raftpb.Entry{{Data: []byte("testdata")}},
		})
		msgs := rnd.readAndClearMailbox()
		if len(msgs) != 1 {
			t.Fatalf("#%d: len(msgs) expected 1, got %d (%+v)", i, len(msgs), msgs)
		}
	}

	// pr2 inflights must be full because proposals above should have filled up all inflights window to 2
	if !pr2.inflights.full() {
		t.Fatalf("inflights.full expected true, got %v", pr2.inflights.full())
	}

	for i := 0; i < 10; i++ {
		rnd.Step(raftpb.Message{
			Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
			From:    1,
			To:      1,
			Entries: []raftpb.Entry{{Data: []byte("testdata")}},
		})
		msgs := rnd.readAndClearMailbox()
		if len(msgs) != 0 {
			t.Fatalf("#%d: len(msgs) expected 0, got %d (%+v)", i, len(msgs), msgs)
		}
	}
}

// (etcd raft.TestMsgAppFlowControlMoveForward)

// (etcd raft.TestMsgAppFlowControlRecvHeartbeat)
