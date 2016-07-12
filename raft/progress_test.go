package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestProgressBecomeProbe)
func Test_Progress_becomeProbe(t *testing.T) {
	tests := []struct {
		pr          *Progress
		wMatchIndex uint64
		wNextIndex  uint64
	}{
		{
			&Progress{State: raftpb.PROGRESS_STATE_REPLICATE, MatchIndex: 1, NextIndex: 5, inflights: newInflights(256)},
			1,
			2, // pr.NextIndex = pr.MatchIndex + 1 // probe next index
		},

		{ // snapshot finish
			&Progress{State: raftpb.PROGRESS_STATE_SNAPSHOT, MatchIndex: 1, NextIndex: 5, PendingSnapshotIndex: 10, inflights: newInflights(256)},
			1,
			11, // pr.NextIndex = maxUint64(pr.MatchIndex+1, lastPendingSnapshotIndex+1)
		},

		{ // snapshot failure
			&Progress{State: raftpb.PROGRESS_STATE_SNAPSHOT, MatchIndex: 1, NextIndex: 5, PendingSnapshotIndex: 0, inflights: newInflights(256)},
			1,
			2, // pr.NextIndex = maxUint64(pr.MatchIndex+1, lastPendingSnapshotIndex+1)
		},
	}

	for i, tt := range tests {
		tt.pr.becomeProbe()
		if tt.pr.State != raftpb.PROGRESS_STATE_PROBE {
			t.Fatalf("#%d: progress state expected %q, got %q", i, raftpb.PROGRESS_STATE_PROBE, tt.pr.State)
		}
		if tt.pr.MatchIndex != tt.wMatchIndex {
			t.Fatalf("#%d: progress match index expected %d, got %d", i, tt.wMatchIndex, tt.pr.MatchIndex)
		}
		if tt.pr.NextIndex != tt.wNextIndex {
			t.Fatalf("#%d: progress next index expected %d, got %d", i, tt.wNextIndex, tt.pr.NextIndex)
		}
	}
}

// (etcd raft.TestProgressBecomeReplicate)
func Test_Progress_becomeReplicate(t *testing.T) {
	pr := &Progress{State: raftpb.PROGRESS_STATE_PROBE, MatchIndex: 1, NextIndex: 5, inflights: newInflights(256)}
	pr.becomeReplicate() // pr.NextIndex = pr.MatchIndex + 1

	if pr.State != raftpb.PROGRESS_STATE_REPLICATE {
		t.Fatalf("progress state expected %q, got %q", raftpb.PROGRESS_STATE_REPLICATE, pr.State)
	}

	if pr.MatchIndex != 1 {
		t.Fatalf("progress match index expected 1, got %d", pr.MatchIndex)
	}

	if w := pr.MatchIndex + 1; pr.NextIndex != w {
		t.Fatalf("progress next index expected %d, got %d", w, pr.NextIndex)
	}
}

// (etcd raft.TestProgressUpdate)
func Test_Progress_maybeUpdateAndResume(t *testing.T) {
	tests := []struct {
		pr             *Progress
		newUpdateIndex uint64

		wMatchIndex, wNextIndex uint64
		wOk                     bool
	}{
		{
			&Progress{MatchIndex: 3, NextIndex: 5},
			2, // never decrease match index

			3, 5,
			false,
		},

		{
			&Progress{MatchIndex: 3, NextIndex: 5},
			3, // never decrease next index

			3, 5,
			false,
		},

		{
			&Progress{MatchIndex: 3, NextIndex: 5},
			4, // increase match, never decrease next index

			4, 5,
			true,
		},

		{
			&Progress{MatchIndex: 3, NextIndex: 5},
			5, // increase match, next

			5, 6,
			true,
		},

		{
			&Progress{MatchIndex: 3, NextIndex: 5},
			7, // increase match, next

			7, 8,
			true,
		},
	}

	for i, tt := range tests {
		// maybeUpdateAndResume returns false if the update index comes from an outdated message.
		//
		// pr.MatchIndex = newUpdateIndex
		// pr.NextIndex = newUpdateIndex + 1
		ok := tt.pr.maybeUpdateAndResume(tt.newUpdateIndex)
		if ok != tt.wOk {
			t.Fatalf("#%d: ok expected %v, got %v", i, tt.wOk, ok)
		}
		if tt.pr.MatchIndex != tt.wMatchIndex {
			t.Fatalf("#%d: progress match index expected %d, got %d", i, tt.wMatchIndex, tt.pr.MatchIndex)
		}
		if tt.pr.NextIndex != tt.wNextIndex {
			t.Fatalf("#%d: progress next index expected %d, got %d", i, tt.wNextIndex, tt.pr.NextIndex)
		}
	}
}

// (etcd raft.TestProgressMaybeDecr)
func Test_Progress_maybeDecreaseAndResume(t *testing.T) {

}

// (etcd raft.TestProgressIsPaused)
func Test_Progress_isPaused(t *testing.T) {

}

// (etcd raft.TestProgressResume)
func Test_Progress_resume(t *testing.T) {

}

// (etcd raft.TestProgressResumeByHeartbeat)
func Test_Progress_becomeLeader_resume_by_heartbeat(t *testing.T) {

}

// (etcd raft.TestProgressPaused)
func Test_Progress_becomeLeader_duplicate_MsgProp(t *testing.T) {

}
