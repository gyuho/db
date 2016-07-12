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
		wPaused                 bool
	}{
		{
			&Progress{MatchIndex: 3, NextIndex: 5},
			2, // never decrease match index

			3, 5,
			false,
			false, // resumes if update index is greater than current match index.
		},

		{
			&Progress{MatchIndex: 3, NextIndex: 5},
			3, // never decrease next index

			3, 5,
			false,
			false, // resumes if update index is greater than current match index.
		},

		{
			&Progress{MatchIndex: 3, NextIndex: 5},
			4, // increase match, never decrease next index

			4, 5,
			true,
			false, // resumes if update index is greater than current match index.
		},

		{
			&Progress{MatchIndex: 3, NextIndex: 5},
			5, // increase match, next

			5, 6,
			true,
			false, // resumes if update index is greater than current match index.
		},

		{
			&Progress{MatchIndex: 3, NextIndex: 5},
			7, // increase match, next

			7, 8,
			true,
			false, // resumes if update index is greater than current match index.
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
		if tt.pr.isPaused() != tt.wPaused {
			t.Fatalf("#%d: paused expected %v, got %v", i, tt.wPaused, tt.pr.isPaused())
		}
	}
}

// (etcd raft.TestProgressMaybeDecr)
func Test_Progress_maybeDecreaseAndResume(t *testing.T) {
	tests := []struct {
		pr                             *Progress
		rejectedLogIndex               uint64
		rejectHintFollowerLogLastIndex uint64

		wMatchIndex, wNextIndex uint64
		wOk                     bool
	}{
		{ // rejected is not greater than match index
			&Progress{State: raftpb.PROGRESS_STATE_REPLICATE, MatchIndex: 5, NextIndex: 10},
			4,
			4,

			5, 10,
			false, // pr.State == raftpb.PROGRESS_STATE_REPLICATE, rejectedLogIndex <= pr.MatchIndex
		},

		{ // rejected is not greater than match index
			&Progress{State: raftpb.PROGRESS_STATE_REPLICATE, MatchIndex: 5, NextIndex: 10},
			5,
			5,

			5, 10,
			false, // pr.State == raftpb.PROGRESS_STATE_REPLICATE, rejectedLogIndex <= pr.MatchIndex
		},

		{ // rejected is greater than match index, decrease the next index
			&Progress{State: raftpb.PROGRESS_STATE_REPLICATE, MatchIndex: 5, NextIndex: 10},
			9,
			9,

			5, 6,
			true, // pr.State == raftpb.PROGRESS_STATE_REPLICATE, rejectedLogIndex > pr.MatchIndex, pr.NextIndex = pr.MatchIndex + 1
		},

		{
			&Progress{State: raftpb.PROGRESS_STATE_PROBE, MatchIndex: 0, NextIndex: 0},
			0,
			0,

			0, 0,
			false, // if pr.NextIndex-1 != rejectedLogIndex { return false }
		},

		{
			&Progress{State: raftpb.PROGRESS_STATE_PROBE, MatchIndex: 0, NextIndex: 10},
			5,
			5,

			0, 10,
			false, // if pr.NextIndex-1 != rejectedLogIndex { return false }
		},

		{
			&Progress{State: raftpb.PROGRESS_STATE_PROBE, MatchIndex: 0, NextIndex: 2},
			1,
			1,

			0, 1,
			true, // pr.NextIndex-1 == rejectedLogIndex, pr.NextIndex = minUint64(rejectedLogIndex, rejectHintFollowerLogLastIndex+1)
		},

		{
			&Progress{State: raftpb.PROGRESS_STATE_PROBE, MatchIndex: 0, NextIndex: 10},
			9,
			9,

			0, 9,
			true, // pr.NextIndex-1 == rejectedLogIndex, pr.NextIndex = minUint64(rejectedLogIndex, rejectHintFollowerLogLastIndex+1)
		},

		{
			&Progress{State: raftpb.PROGRESS_STATE_PROBE, MatchIndex: 0, NextIndex: 1},
			0,
			0,

			0, 1,
			true, // pr.NextIndex-1 == rejectedLogIndex, pr.NextIndex = minUint64(rejectedLogIndex, rejectHintFollowerLogLastIndex+1)
			// if pr.NextIndex < 1 { pr.NextIndex = 1 }
		},

		{
			&Progress{State: raftpb.PROGRESS_STATE_PROBE, MatchIndex: 0, NextIndex: 10},
			9,
			2,

			0, 3,
			true, // pr.NextIndex-1 == rejectedLogIndex, pr.NextIndex = minUint64(rejectedLogIndex, rejectHintFollowerLogLastIndex+1)
		},

		{
			&Progress{State: raftpb.PROGRESS_STATE_PROBE, MatchIndex: 0, NextIndex: 10},
			9,
			0,

			0, 1,
			true, // pr.NextIndex-1 == rejectedLogIndex, pr.NextIndex = minUint64(rejectedLogIndex, rejectHintFollowerLogLastIndex+1)
			// if pr.NextIndex < 1 { pr.NextIndex = 1 }
		},
	}

	for i, tt := range tests {
		// maybeDecreaseAndResume returns true if the rejecting message's log index
		// comes from an outdated message. Otherwise, it decreases the next
		// index in the follower's progress, and returns true.
		if ok := tt.pr.maybeDecreaseAndResume(tt.rejectedLogIndex, tt.rejectHintFollowerLogLastIndex); ok != tt.wOk {
			t.Fatalf("#%d: maybeDecreaseAndResume ok expected %v, got %v", i, tt.wOk, ok)
		}
		if tt.pr.MatchIndex != tt.wMatchIndex {
			t.Fatalf("#%d: progress match index expected %d, got %d", i, tt.wMatchIndex, tt.pr.MatchIndex)
		}
		if tt.pr.NextIndex != tt.wNextIndex {
			t.Fatalf("#%d: progress next index expected %d, got %d", i, tt.wNextIndex, tt.pr.NextIndex)
		}
	}
}

// (etcd raft.TestProgressIsPaused)
func Test_Progress_isPaused(t *testing.T) {
	tests := []struct {
		pr      *Progress
		wPaused bool
	}{
		{
			&Progress{State: raftpb.PROGRESS_STATE_PROBE, Paused: false, inflights: newInflights(256)},
			false, // case raftpb.PROGRESS_STATE_PROBE: return pr.Paused
		},

		{
			&Progress{State: raftpb.PROGRESS_STATE_PROBE, Paused: true, inflights: newInflights(256)},
			true, // case raftpb.PROGRESS_STATE_PROBE: return pr.Paused
		},

		{
			&Progress{State: raftpb.PROGRESS_STATE_REPLICATE, Paused: false, inflights: newInflights(256)},
			false, // case raftpb.PROGRESS_STATE_REPLICATE: return pr.inflights.full(), len(ins.buffer) == ins.bufferCount
		},

		{
			&Progress{State: raftpb.PROGRESS_STATE_REPLICATE, Paused: true, inflights: newInflights(256)},
			false, // case raftpb.PROGRESS_STATE_REPLICATE: return pr.inflights.full(), len(ins.buffer) == ins.bufferCount
		},

		{
			&Progress{State: raftpb.PROGRESS_STATE_SNAPSHOT, Paused: false, inflights: newInflights(256)},
			true, // case raftpb.PROGRESS_STATE_SNAPSHOT: return true
		},

		{
			&Progress{State: raftpb.PROGRESS_STATE_SNAPSHOT, Paused: true, inflights: newInflights(256)},
			true, // case raftpb.PROGRESS_STATE_SNAPSHOT: return true
		},
	}

	for i, tt := range tests {
		if paused := tt.pr.isPaused(); paused != tt.wPaused {
			t.Fatalf("#%d: paused expected %v, got %v", i, tt.wPaused, paused)
		}
	}
}

// (etcd raft.TestProgressResume)
func Test_Progress_resume(t *testing.T) {
	pr := &Progress{MatchIndex: 0, NextIndex: 2, Paused: true}

	// pr.NextIndex-1 == rejectedLogIndex
	//
	// pr.NextIndex = minUint64(rejectedLogIndex, rejectHintFollowerLogLastIndex+1)
	//              = minUint64(1, 2) = 1
	//
	// pr.resume()
	if ok := pr.maybeDecreaseAndResume(1, 1); !ok {
		t.Fatalf("ok expected true, got %v", ok)
	}
	if pr.Paused {
		t.Fatalf("after maybeDecreaseAndResume, it should have been resumed with paused false, got %v", pr.Paused)
	}

	pr.Paused = true

	// pr.MatchIndex < newUpdateIndex
	// pr.MatchIndex = newUpdateIndex
	// upToDate = true
	// pr.true
	if ok := pr.maybeUpdateAndResume(2); !ok {
		t.Fatalf("maybeUpdateAndResume expected true, got %v", ok)
	}
	if pr.Paused {
		t.Fatalf("after maybeUpdateAndResume, it should have been resumed with paused false, got %v", pr.Paused)
	}
}

// (etcd raft.TestProgressResumeByHeartbeat)
func Test_Progress_becomeLeader_resume_by_heartbeat(t *testing.T) {

}

// (etcd raft.TestProgressPaused)
func Test_Progress_becomeLeader_duplicate_MsgProp(t *testing.T) {

}
