package raft

import (
	"fmt"

	"github.com/gyuho/db/raft/raftpb"
)

// FollowerProgress is follower's state in leader's view.
type FollowerProgress struct {
	// State is either PROBE, REPLICATE, SNAPSHOT.
	State raftpb.FOLLOWER_PROGRESS_STATE

	// MatchIndex is the highest known matched entry index
	// of this follower.
	//
	// (etcd raft.Progress.Match)
	MatchIndex uint64

	// NextIndex is the starting index of entries
	// for next replication.
	//
	// (etcd raft.Progress.Next)
	NextIndex uint64

	// PendingSnapshotIndex is used in SNAPSHOT state.
	// PendingSnapshotIndex is the index of the ongoing snapshot.
	// When PendingSnapshotIndex is set, leader stops replication
	// to this follower.
	//
	// (etcd raft.Progress.PendingSnapshot)
	PendingSnapshotIndex uint64

	// Paused is used in PROBE state.
	// When Paused is true, leader stops sending replication messages
	// to this follower.
	//
	// (etcd raft.Progress.Paused)
	Paused bool

	// Active is true if this follower is recently active,
	// such as receiving any message from this follower.
	// It can be reset to false after election timeout.
	//
	// (etcd raft.Progress.RecentActive)
	Active bool

	// inflights represents the status of buffered messages
	// to this follower. When it's full, no more messages should
	// be sent to this follower.
	inflights *inflights
}

func (fpr *FollowerProgress) resetState(state raftpb.FOLLOWER_PROGRESS_STATE) {
	fpr.State = state
	fpr.PendingSnapshotIndex = 0
	fpr.Paused = false
	fpr.Active = false
}

func (fpr *FollowerProgress) becomeProbe() {
	if fpr.State == raftpb.FOLLOWER_PROGRESS_STATE_SNAPSHOT { // snapshot was sent
		pIdx := fpr.PendingSnapshotIndex
		fpr.resetState(raftpb.FOLLOWER_PROGRESS_STATE_PROBE)
		fpr.NextIndex = maxUint64(fpr.MatchIndex+1, pIdx+1)
		return
	}
	fpr.resetState(raftpb.FOLLOWER_PROGRESS_STATE_PROBE)
	fpr.NextIndex = fpr.MatchIndex + 1 // probe next index
}

func (fpr *FollowerProgress) becomeReplicate() {
	fpr.resetState(raftpb.FOLLOWER_PROGRESS_STATE_REPLICATE)
	fpr.NextIndex = fpr.MatchIndex + 1 // probe next index
}

func (fpr *FollowerProgress) pause() {
	fpr.Paused = true
}

func (fpr *FollowerProgress) resume() {
	fpr.Paused = false
}

func (fpr *FollowerProgress) isPaused() bool {
	switch fpr.State {
	case raftpb.FOLLOWER_PROGRESS_STATE_PROBE:
		return fpr.Paused
	case raftpb.FOLLOWER_PROGRESS_STATE_REPLICATE:
		return fpr.inflights.full()
	case raftpb.FOLLOWER_PROGRESS_STATE_SNAPSHOT:
		return true
	default:
		raftLogger.Panicf("unexpected Progress.State %q", fpr.State)
		return true
	}
}

func (fpr *FollowerProgress) becomeSnapshot(snapshotIndex uint64) {
	fpr.resetState(raftpb.FOLLOWER_PROGRESS_STATE_SNAPSHOT)
	fpr.PendingSnapshotIndex = snapshotIndex
}

func (fpr *FollowerProgress) optimisticUpdate(msgLogIndex uint64) {
	fpr.NextIndex = msgLogIndex + 1
}

// maybeUpdate returns false if the given index comes from an
// outdated message.
//
// (etcd raft.Progress.maybeUpdate)
func (fpr *FollowerProgress) maybeUpdate(msgLogIndex uint64) bool {
	upToDate := false
	if fpr.MatchIndex < msgLogIndex { // update MatchIndex
		fpr.MatchIndex = msgLogIndex
		upToDate = true
		fpr.resume()
	}

	if fpr.NextIndex <= msgLogIndex { // update NextIndex
		fpr.NextIndex = msgLogIndex + 1
	}

	return upToDate
}

// maybeDecrease returns true if the rejecting message's log index
// comes from an outdated message. Otherwise, it decreases the next
// index in the follower's progress, and returns true.
//
// (etcd raft.Progress.maybeDecrTo)
func (fpr *FollowerProgress) maybeDecrease(rejectLogIndex, rejectHint uint64) bool {
	if fpr.State == raftpb.FOLLOWER_PROGRESS_STATE_REPLICATE {
		if rejectLogIndex <= fpr.MatchIndex {
			return false
		}

		fpr.NextIndex = fpr.MatchIndex + 1
		return true
	}

	if fpr.NextIndex-1 != rejectLogIndex {
		return false
	}

	fpr.NextIndex = minUint64(rejectLogIndex, rejectHint+1)
	if fpr.NextIndex < 1 {
		fpr.NextIndex = 1
	}

	fpr.resume()
	return true
}

// needSnapshotAbort returns true if it needs to stop sending snapshot to the
// follower.
func (fpr *FollowerProgress) needSnapshotAbort() bool {
	return fpr.State == raftpb.FOLLOWER_PROGRESS_STATE_SNAPSHOT && fpr.MatchIndex >= fpr.PendingSnapshotIndex
}

func (fpr *FollowerProgress) snapshotFailed() {
	fpr.PendingSnapshotIndex = 0 // reset because it failed
}

func (fpr *FollowerProgress) String() string {
	return fmt.Sprintf("[state=%q | match index=%d | next index=%d | paused(waiting)=%v | pending Snapshot index=%d]",
		fpr.State,
		fpr.MatchIndex,
		fpr.NextIndex,
		fpr.isPaused(),
		fpr.PendingSnapshotIndex,
	)
}
