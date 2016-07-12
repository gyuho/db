package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestProgressBecomeProbe)
func Test_Progress_becomeProbe(t *testing.T) {
	tests := []struct {
		progress    *Progress
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
		tt.progress.becomeProbe()
		if tt.progress.State != raftpb.PROGRESS_STATE_PROBE {
			t.Fatalf("#%d: progress state expected %q, got %q", i, raftpb.PROGRESS_STATE_PROBE, tt.progress.State)
		}
		if tt.progress.MatchIndex != tt.wMatchIndex {
			t.Fatalf("#%d: progress match index expected %d, got %d", i, tt.wMatchIndex, tt.progress.MatchIndex)
		}
		if tt.progress.NextIndex != tt.wNextIndex {
			t.Fatalf("#%d: progress next index expected %d, got %d", i, tt.wNextIndex, tt.progress.NextIndex)
		}
	}
}
