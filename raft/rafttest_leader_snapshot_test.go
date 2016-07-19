package raft

import (
	"reflect"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestBcastBeat)
func Test_raft_snapshot_heartbeat(t *testing.T) {
	snap := raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			Index:       1000,
			Term:        1,
			ConfigState: raftpb.ConfigState{IDs: []uint64{1, 2, 3}},
		},
	}
	st := NewStorageStableInMemory()
	st.ApplySnapshot(snap)
	rnd := newTestRaftNode(1, nil, 10, 1, st)
	rnd.term = snap.Metadata.Term

	if !reflect.DeepEqual(rnd.allNodeIDs(), []uint64{1, 2, 3}) {
		t.Fatalf("node ids expected %+v, got %+v", []uint64{1, 2, 3}, rnd.allNodeIDs())
	}

	rnd.becomeCandidate()
	rnd.becomeLeader()

	for i := 0; i < 10; i++ {
		rnd.leaderAppendEntriesToLeader(raftpb.Entry{Index: uint64(i) + 1})
	}

	// slow follower
	rnd.allProgresses[2].MatchIndex = 5
	rnd.allProgresses[2].NextIndex = 6

	// normal follower
	rnd.allProgresses[3].MatchIndex = rnd.storageRaftLog.lastIndex()
	rnd.allProgresses[3].NextIndex = rnd.storageRaftLog.lastIndex() + 1

	// trigger leader to send heartbeat
	rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT, From: 1, To: 1})

	msgs := rnd.readAndClearMailbox()
	if len(msgs) != 2 { // heartbeats from 1 to 2, 3
		t.Fatalf("len(msgs) expected 2, got %d", len(msgs))
	}

	for i, msg := range msgs {
		if msg.Type != raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT {
			t.Fatalf("#%d: msg.Type expected %q, got %q", i, raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT, msg.Type)
		}
		if msg.LogIndex != 0 {
			t.Fatalf("#%d: msg.LogIndex expected 0, got %d", i, msg.LogIndex)
		}
		if msg.LogTerm != 0 {
			t.Fatalf("#%d: msg.LogTerm expected 0, got %d", i, msg.LogTerm)
		}

		if len(msg.Entries) != 0 {
			t.Fatalf("#%d: len(msg.Entries) expected 0, got %d", i, len(msg.Entries))
		}

		// leaderSendHeartbeatTo
		targetID := msg.To
		var (
			matched         = rnd.allProgresses[targetID].MatchIndex
			commitInStorage = rnd.storageRaftLog.committedIndex
			committedIndex  = minUint64(matched, commitInStorage)
		)
		if msg.SenderCurrentCommittedIndex != committedIndex {
			t.Fatalf("#%d: msg.SenderCurrentCommittedIndex expected %d, got %d", i, committedIndex, msg.SenderCurrentCommittedIndex)
		}
	}
}

// (etcd raft.TestSendingSnapshotSetPendingSnapshot)
func Test_raft_snapshot_restoreSnapshot_pending_snapshot(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())
	rnd.restoreSnapshot(raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			Index:       11,
			Term:        11,
			ConfigState: raftpb.ConfigState{IDs: []uint64{1, 2}},
		},
	})

	rnd.becomeCandidate()
	rnd.becomeLeader()

	// resetWithTerm updates all progresses
	// NextIndex: rnd.storageRaftLog.lastIndex() + 1,

	// to force 1 to send snapshot to 2
	rnd.allProgresses[2].NextIndex = rnd.storageRaftLog.firstIndex()
	rnd.Step(raftpb.Message{
		Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND,
		From:     2,
		To:       1,
		LogIndex: rnd.allProgresses[2].NextIndex - 1,
		Reject:   true,
	})

	if rnd.allProgresses[2].PendingSnapshotIndex != 11 {
		t.Fatalf("rnd.allProgresses[2].PendingSnapshotIndex expected 11, got %d", rnd.allProgresses[2].PendingSnapshotIndex)
	}
}

// (etcd raft.TestPendingSnapshotPauseReplication)
func Test_raft_snapshot_pause_replication(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())
	rnd.restoreSnapshot(raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			Index:       11,
			Term:        11,
			ConfigState: raftpb.ConfigState{IDs: []uint64{1, 2}},
		},
	})

	rnd.becomeCandidate()
	rnd.becomeLeader()

	rnd.allProgresses[2].becomeSnapshot(11)

	rnd.Step(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		From:    1,
		To:      1,
		Entries: []raftpb.Entry{{Data: []byte("testdata")}},
	})

	msgs := rnd.readAndClearMailbox()
	if len(msgs) != 0 {
		t.Fatalf("len(msgs) expected 0, got %d", len(msgs))
	}
	/*
	   followerProgress := rnd.allProgresses[targetID]
	   if followerProgress.isPaused() { // snapshot returns true
	   	raftLogger.Debugf("%s skips append/snapshot to paused follower %x", rnd.describe(), targetID)
	   	return
	   }
	*/
}

// (etcd raft.TestSnapshotFailure)
func Test_raft_snapshot_failure(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())
	rnd.restoreSnapshot(raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			Index:       11,
			Term:        11,
			ConfigState: raftpb.ConfigState{IDs: []uint64{1, 2}},
		},
	})

	rnd.becomeCandidate()
	rnd.becomeLeader()

	rnd.allProgresses[2].NextIndex = 1
	rnd.allProgresses[2].becomeSnapshot(11)

	rnd.Step(raftpb.Message{
		Type:   raftpb.MESSAGE_TYPE_INTERNAL_RESPONSE_TO_LEADER_SNAPSHOT,
		From:   2,
		To:     1,
		Reject: true,
	})
	// followerProgress.snapshotFailed() // set pending snapshot index to 0
	// followerProgress.becomeProbe()
	// followerProgress.pause()

	if rnd.allProgresses[2].PendingSnapshotIndex != 0 {
		t.Fatalf("rnd.allProgresses[2].PendingSnapshotIndex expected 0, got %d", rnd.allProgresses[2].PendingSnapshotIndex)
	}
	if rnd.allProgresses[2].NextIndex != 1 {
		t.Fatalf("rnd.allProgresses[2].NextIndex expected 1, got %d", rnd.allProgresses[2].NextIndex)
	}
	if !rnd.allProgresses[2].isPaused() {
		t.Fatalf("rnd.allProgresses[2].isPaused() expected true, got %v", rnd.allProgresses[2].isPaused())
	}
}

// (etcd raft.TestSnapshotSucceed)
func Test_raft_snapshot_succeed(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())
	rnd.restoreSnapshot(raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			Index:       11,
			Term:        11,
			ConfigState: raftpb.ConfigState{IDs: []uint64{1, 2}},
		},
	})

	rnd.becomeCandidate()
	rnd.becomeLeader()

	rnd.allProgresses[2].NextIndex = 1
	rnd.allProgresses[2].becomeSnapshot(11)

	rnd.Step(raftpb.Message{
		Type:   raftpb.MESSAGE_TYPE_INTERNAL_RESPONSE_TO_LEADER_SNAPSHOT,
		From:   2,
		To:     1,
		Reject: false,
	})
	// followerProgress.becomeProbe()
	// followerProgress.pause()
	//
	// becomeProbe:
	// if pr.State == raftpb.PROGRESS_STATE_SNAPSHOT { // snapshot was sent
	// 	lastPendingSnapshotIndex := pr.PendingSnapshotIndex
	// 	pr.resetState(raftpb.PROGRESS_STATE_PROBE)
	// 	pr.NextIndex = maxUint64(pr.MatchIndex+1, lastPendingSnapshotIndex+1)
	// 	return
	// }

	if rnd.allProgresses[2].PendingSnapshotIndex != 0 {
		t.Fatalf("rnd.allProgresses[2].PendingSnapshotIndex expected 0, got %d", rnd.allProgresses[2].PendingSnapshotIndex)
	}
	if rnd.allProgresses[2].NextIndex != 12 {
		t.Fatalf("rnd.allProgresses[2].NextIndex expected 12, got %d", rnd.allProgresses[2].NextIndex)
	}
	if !rnd.allProgresses[2].isPaused() {
		t.Fatalf("rnd.allProgresses[2].isPaused() expected true, got %v", rnd.allProgresses[2].isPaused())
	}
}

// (etcd raft.TestSnapshotAbort)
func Test_raft_snapshot_abort(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())
	rnd.restoreSnapshot(raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			Index:       11,
			Term:        11,
			ConfigState: raftpb.ConfigState{IDs: []uint64{1, 2}},
		},
	})

	rnd.becomeCandidate()
	rnd.becomeLeader()

	rnd.allProgresses[2].NextIndex = 1
	rnd.allProgresses[2].becomeSnapshot(11)

	rnd.Step(raftpb.Message{
		Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND,
		From:     2,
		To:       1,
		LogIndex: 11,
	})
	// case raftpb.PROGRESS_STATE_SNAPSHOT:
	// 		if followerProgress.needSnapshotAbort() { // pr.MatchIndex >= pr.PendingSnapshotIndex
	// 			followerProgress.becomeProbe()
	// 			raftLogger.Infof("%s is stopping snapshot to follower %x, and resetting progress to %s", rnd.describe(), msg.From, followerProgress)
	// 		}
	// 	}

	if rnd.allProgresses[2].PendingSnapshotIndex != 0 {
		t.Fatalf("rnd.allProgresses[2].PendingSnapshotIndex expected 0, got %d", rnd.allProgresses[2].PendingSnapshotIndex)
	}
	if rnd.allProgresses[2].NextIndex != 12 {
		t.Fatalf("rnd.allProgresses[2].NextIndex expected 12, got %d", rnd.allProgresses[2].NextIndex)
	}
	if !rnd.allProgresses[2].isPaused() {
		t.Fatalf("rnd.allProgresses[2].isPaused() expected true, got %v", rnd.allProgresses[2].isPaused())
	}
}

// (etcd raft.TestRestore)
func Test_raft_snapshot_restore(t *testing.T) {
	snap := raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			Index:       11,
			Term:        11,
			ConfigState: raftpb.ConfigState{IDs: []uint64{1, 2, 3}},
		},
	}

	rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())
	if ok := rnd.restoreSnapshot(snap); !ok {
		t.Fatalf("restoreSnapshot expected true, got %v", ok)
	}

	if !reflect.DeepEqual(rnd.allNodeIDs(), snap.Metadata.ConfigState.IDs) {
		t.Fatalf("all node ids expected %+v, got %+v", snap.Metadata.ConfigState.IDs, rnd.allNodeIDs())
	}

	if rnd.storageRaftLog.lastIndex() != snap.Metadata.Index {
		t.Fatalf("rnd.storageRaftLog.lastIndex() expected %d, got %d", snap.Metadata.Index, rnd.storageRaftLog.lastIndex())
	}

	term, err := rnd.storageRaftLog.term(snap.Metadata.Index)
	if err != nil {
		t.Fatal(err)
	}
	if term != snap.Metadata.Term {
		t.Fatalf("term expected %d, got %d", snap.Metadata.Term, term)
	}

	if ok := rnd.restoreSnapshot(snap); ok {
		t.Fatalf("restoreSnapshot expected false, got %v", ok)
	}
	//
	// if rnd.storageRaftLog.committedIndex >= snap.Metadata.Index {
	// 	return false
	// }
}

// (etcd raft.TestRestoreIgnoreSnapshot)
func Test_raft_snapshot_restore_ignore(t *testing.T) {
	prevEntries := []raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 1}, {Index: 3, Term: 1}}

	rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())
	rnd.storageRaftLog.appendToStorageUnstable(prevEntries...)
	rnd.storageRaftLog.commitTo(1)

	snap := raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			Index:       1,
			Term:        1,
			ConfigState: raftpb.ConfigState{IDs: []uint64{1, 2}},
		},
	}

	// ignore snapshot
	if ok := rnd.restoreSnapshot(snap); ok {
		t.Fatalf("restore expected false, got %v", ok)
	}
	if rnd.storageRaftLog.committedIndex != 1 {
		t.Fatalf("committed index expected 1, got %d", rnd.storageRaftLog.committedIndex)
	}

	// ignore snapshot and fast-forward commit
	snap.Metadata.Index = 2
	if ok := rnd.restoreSnapshot(snap); ok {
		t.Fatalf("restore expected false, got %v", ok)
	}
	if rnd.storageRaftLog.committedIndex != 2 {
		t.Fatalf("committed index expected 2, got %d", rnd.storageRaftLog.committedIndex)
	}
}

// (etcd raft.TestProvideSnap)
func Test_raft_snapshot_restore_leader(t *testing.T) {
	snap := raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			Index:       11,
			Term:        11,
			ConfigState: raftpb.ConfigState{IDs: []uint64{1, 2}},
		},
	}

	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())
	rnd.restoreSnapshot(snap)

	rnd.becomeCandidate()
	rnd.becomeLeader()

	// force set the next index of 2
	// to make it need snapshot from leader
	rnd.allProgresses[2].NextIndex = rnd.storageRaftLog.firstIndex()

	rnd.Step(raftpb.Message{
		Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND,
		From:     2,
		To:       1,
		LogIndex: rnd.allProgresses[2].NextIndex - 1,
		Reject:   true,
	})

	msgs := rnd.readAndClearMailbox()
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) expected 1, got %d", len(msgs))
	}
	if msgs[0].Type != raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT {
		t.Fatalf("msgs[0].Type expected %q, got %q", raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT, msgs[0].Type)
	}
}

// (etcd raft.TestIgnoreProvidingSnap)
func Test_raft_snapshot_restore_leader_cancel_snapshot(t *testing.T) {
	snap := raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			Index:       11,
			Term:        11,
			ConfigState: raftpb.ConfigState{IDs: []uint64{1, 2}},
		},
	}

	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())
	rnd.restoreSnapshot(snap)

	rnd.becomeCandidate()
	rnd.becomeLeader()

	// force set the next index of 2
	// to make it need snapshot from leader
	rnd.allProgresses[2].NextIndex = rnd.storageRaftLog.firstIndex() - 1
	rnd.allProgresses[2].RecentActive = false
	// now node 1 does not send snapshot to 2

	rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, From: 1, To: 1, Entries: []raftpb.Entry{{Data: []byte("testdata")}}})
	// if !followerProgress.RecentActive { return }

	msgs := rnd.readAndClearMailbox()
	if len(msgs) != 0 {
		t.Fatalf("len(msgs) expected 0, got %d", len(msgs))
	}
}

// (etcd raft.TestRestoreFromSnapMsg)
func Test_raft_snapshot_restore_msg_snap(t *testing.T) {

}

// (etcd raft.TestSlowNodeRestore)
func Test_raft_snapshot_restore_slow_node(t *testing.T) {

}
