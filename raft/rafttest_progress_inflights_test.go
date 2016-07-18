package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestMsgAppFlowControlFull)
func Test_raft_progress_inflights_full(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())
	rnd.becomeCandidate()
	rnd.becomeLeader()

	pr2 := rnd.allProgresses[2]

	// force 2 progress to be replicate
	pr2.becomeReplicate()

	// fill in inflights window
	for i := 0; i < rnd.maxInflightMsgNum; i++ {
		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, From: 1, To: 1, Entries: []raftpb.Entry{{Data: []byte("testdata")}}})

		msgs := rnd.readAndClearMailbox()
		if len(msgs) != 1 {
			t.Fatalf("#%d: len(msgs) expected 1, got %d (%+v)", i, len(msgs), msgs)
		}
	}

	// pr2 inflights must be full because proposals above should have filled up all inflights window to 2
	if !pr2.inflights.full() {
		t.Fatalf("inflights.full expected true, got %v", pr2.inflights.full())
	}

	// ensure it's full
	for i := 0; i < 10; i++ {
		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, From: 1, To: 1, Entries: []raftpb.Entry{{Data: []byte("testdata")}}})

		msgs := rnd.readAndClearMailbox()
		if len(msgs) != 0 {
			t.Fatalf("#%d: len(msgs) expected 0, got %d (%+v)", i, len(msgs), msgs)
		}
	}
}

// (etcd raft.TestMsgAppFlowControlMoveForward)
func Test_raft_progress_inflights_move_forward(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())
	rnd.becomeCandidate()
	rnd.becomeLeader()

	pr2 := rnd.allProgresses[2]

	// force 2 progress to be replicate
	pr2.becomeReplicate()

	// fill in inflights window
	for i := 0; i < rnd.maxInflightMsgNum; i++ {
		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, From: 1, To: 1, Entries: []raftpb.Entry{{Data: []byte("testdata")}}})

		msgs := rnd.readAndClearMailbox()
		if len(msgs) != 1 {
			t.Fatalf("#%d: len(msgs) expected 1, got %d (%+v)", i, len(msgs), msgs)
		}
	}

	// index 1 is no-op empty entry, 2 is the first proposal
	// so start from 2
	for i := 2; i < rnd.maxInflightMsgNum; i++ {
		// move forward the progress window
		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_APPEND_FROM_LEADER, From: 2, To: 1, LogIndex: uint64(i)})

		// wasPaused is true, so it will resume again
		/*
			case raftpb.PROGRESS_STATE_REPLICATE:
				// succeed, so free up to entries <= msg.LogIndex
				followerProgress.inflights.freeTo(msg.LogIndex)
		*/

		rnd.readAndClearMailbox()

		// fill in inflights again
		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, From: 1, To: 1, Entries: []raftpb.Entry{{Data: []byte("testdata")}}})

		msgs := rnd.readAndClearMailbox()
		if len(msgs) != 1 {
			t.Fatalf("#%d: len(msgs) expected 1, got %d (%+v)", i, len(msgs), msgs)
		}

		// pr2 inflights must be full because proposals above should have filled up all inflights window to 2
		if !pr2.inflights.full() {
			t.Fatalf("#%d: inflights.full expected true, got %v", i, pr2.inflights.full())
		}

		// ensure it's full
		for j := 0; j < i; j++ {
			rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_APPEND_FROM_LEADER, From: 2, To: 1, LogIndex: uint64(j)})

			if !pr2.inflights.full() {
				t.Fatalf("#%d.%d: inflights.full expected true, got %v", i, j, pr2.inflights.full())
			}
		}
	}
}

// (etcd raft.TestMsgAppFlowControlRecvHeartbeat)
func Test_raft_progress_inflights_full_heartbeat(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())
	rnd.becomeCandidate()
	rnd.becomeLeader()

	pr2 := rnd.allProgresses[2]

	// force 2 progress to be replicate
	pr2.becomeReplicate()

	// fill in inflights window
	for i := 0; i < rnd.maxInflightMsgNum; i++ {
		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, From: 1, To: 1, Entries: []raftpb.Entry{{Data: []byte("testdata")}}})

		msgs := rnd.readAndClearMailbox()
		if len(msgs) != 1 {
			t.Fatalf("#%d: len(msgs) expected 1, got %d (%+v)", i, len(msgs), msgs)
		}
	}

	for i := 1; i < 5; i++ {
		// pr2 inflights must be full because proposals above should have filled up all inflights window to 2
		if !pr2.inflights.full() {
			t.Fatalf("#%d: inflights.full expected true, got %v", i, pr2.inflights.full())
		}

		for j := 0; j < i; j++ {
			rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT, From: 2, To: 1}) // first response will free first one
			rnd.readAndClearMailbox()

			if pr2.inflights.full() {
				t.Fatalf("#%d.%d: inflights.full expected false, got %v", i, j, pr2.inflights.full())
			}
		}

		// one slot
		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, From: 1, To: 1, Entries: []raftpb.Entry{{Data: []byte("testdata")}}})

		msgs := rnd.readAndClearMailbox()
		if len(msgs) != 1 {
			t.Fatalf("#%d: len(msgs) expected 1, got %d (%+v)", i, len(msgs), msgs)
		}

		// just one slot
		for j := 0; j < 10; j++ {
			rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, From: 1, To: 1, Entries: []raftpb.Entry{{Data: []byte("testdata")}}})

			msgs := rnd.readAndClearMailbox()
			if len(msgs) != 0 {
				t.Fatalf("#%d.%d: len(msgs) expected 0, got %d (%+v)", i, j, len(msgs), msgs)
			}
		}

		// clear pending messages
		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT, From: 2, To: 1})
		rnd.readAndClearMailbox()
	}
}
