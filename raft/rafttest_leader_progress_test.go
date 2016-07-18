package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestMsgAppFlowControlFull)
func Test_raft_leader_progress_inflights_full(t *testing.T) {
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
func Test_raft_leader_progress_inflights_move_forward(t *testing.T) {
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
func Test_raft_leader_progress_inflights_full_heartbeat(t *testing.T) {
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

// (etcd raft.TestLeaderIncreaseNext)
func Test_raft_leader_progress_increase_next_index(t *testing.T) {
	prevEntries := []raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 1}, {Index: 3, Term: 1}}

	tests := []struct {
		progressState raftpb.PROGRESS_STATE
		nextIndex     uint64

		wNextIndex uint64
	}{
		{
			/*
				case raftpb.PROGRESS_STATE_REPLICATE:
					lastIndex := msg.Entries[len(msg.Entries)-1].Index
					followerProgress.optimisticUpdate(lastIndex)
					followerProgress.inflights.add(lastIndex)

				func (pr *Progress) optimisticUpdate(msgLogIndex uint64) {
					pr.NextIndex = msgLogIndex + 1
				}
			*/
			raftpb.PROGRESS_STATE_REPLICATE,
			2,

			// 3 entries from prevEntries + 1 from becomeLeader
			/// 1 entry from proposal
			// so, last index is 5
			6,
		},

		{
			raftpb.PROGRESS_STATE_PROBE,
			2,

			2,
		},
	}

	for i, tt := range tests {
		rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())
		rnd.storageRaftLog.appendToStorageUnstable(prevEntries...)
		rnd.becomeCandidate()
		rnd.becomeLeader()

		pr := rnd.allProgresses[2]
		pr.State = tt.progressState
		pr.NextIndex = tt.nextIndex

		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, From: 1, To: 1, Entries: []raftpb.Entry{{Data: []byte("testdata")}}})

		if pr.NextIndex != tt.wNextIndex {
			t.Fatalf("#%d: next index expected %d, got %d", i, tt.wNextIndex, pr.NextIndex)
		}
	}
}

// (etcd raft.TestSendAppendForProgressProbe)
func Test_raft_leader_progress_append_to_progress_probe(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())
	rnd.becomeCandidate()
	rnd.becomeLeader()
	rnd.readAndClearMailbox()

	// empty no-op entry
	ents := rnd.storageRaftLog.unstableEntries()
	if len(ents) != 1 {
		t.Fatalf("len(ents) expected 1, got %+v", ents)
	}
	if len(ents[0].Data) != 0 {
		t.Fatalf("len(ents[0].Entries) expected 0, got %+v", ents)
	}

	rnd.allProgresses[2].becomeProbe()

	for i := 0; i < 3; i++ {
		rnd.leaderAppendEntriesToLeader(raftpb.Entry{Data: []byte("testdata")})
		rnd.leaderSendAppendOrSnapshot(2)

		msgs := rnd.readAndClearMailbox()
		if len(msgs) != 1 {
			t.Fatalf("#%d: len(msgs) expected 1, got %d", i, len(msgs))
		}
		if msgs[0].LogIndex != 0 {
			t.Fatalf("#%d: msgs[0].LogIndex expected 0, got %d", i, msgs[0].LogIndex)
		}

		if !rnd.allProgresses[2].isPaused() {
			t.Fatalf("#%d: rnd.allProgresses[2].isPaused() expected true, got %v", i, rnd.allProgresses[2].isPaused())
		}

		for j := 0; j < 10; j++ {
			rnd.leaderAppendEntriesToLeader(raftpb.Entry{Data: []byte("testdata")})
			rnd.leaderSendAppendOrSnapshot(2)
			if msgs := rnd.readAndClearMailbox(); len(msgs) != 0 { // because it's probe and paused
				t.Fatalf("#%d.%d: len(msgs) expected 0, got %d", i, j, len(msgs))
			}
		}

		// trigger leader heartbeat
		for j := 0; j < rnd.heartbeatTimeoutTickNum; j++ {
			rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT, From: 1, To: 1})
		}
		// rnd.leaderReplicateHeartbeatRequests() // resume all progresses

		if rnd.allProgresses[2].isPaused() {
			t.Fatalf("#%d: rnd.allProgresses[2].isPaused() expected false, got %v", i, rnd.allProgresses[2].isPaused())
		}
		msgs = rnd.readAndClearMailbox()
		if len(msgs) != 1 {
			t.Fatalf("#%d: len(msgs) expected 1, got %d", i, len(msgs))
		}
		if msgs[0].Type != raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT {
			t.Fatalf("#%d: msgs[0].Type expected %q, got %q", i, raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT, msgs[0].Type)
		}
	}
}

// (etcd raft.TestSendAppendForProgressReplicate)
func Test_raft_leader_progress_append_to_progress_replicate(t *testing.T) {

}

// (etcd raft.TestSendAppendForProgressSnapshot)
func Test_raft_leader_progress_append_to_progress_snapshot(t *testing.T) {

}

// (etcd raft.TestRecvMsgUnreachable)
func Test_raft_leader_progress_unreachable(t *testing.T) {

}
