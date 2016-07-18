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
func Test_raft_snapshot_followerRestoreSnapshot_pending_snapshot(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())
	rnd.followerRestoreSnapshot(raftpb.Snapshot{
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
		Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_APPEND_FROM_LEADER,
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
	rnd.followerRestoreSnapshot(raftpb.Snapshot{
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

// (etcd raft.TestSnapshotSucceed)

// (etcd raft.TestSnapshotAbort)

// (etcd raft.TestRestore)

// (etcd raft.TestRestoreIgnoreSnapshot)

// (etcd raft.TestProvideSnap)

// (etcd raft.TestIgnoreProvidingSnap)

// (etcd raft.TestRestoreFromSnapMsg)

// (etcd raft.TestSlowNodeRestore)
