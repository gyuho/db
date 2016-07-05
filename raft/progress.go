package raft

import (
	"fmt"

	"github.com/gyuho/db/raft/raftpb"
)

// Progress is follower's state in leader's view.
type Progress struct {
	// State is either PROBE, REPLICATE, SNAPSHOT.
	State raftpb.PROGRESS_STATE

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

func (pr *Progress) resetState(state raftpb.PROGRESS_STATE) {
	pr.State = state
	pr.PendingSnapshotIndex = 0
	pr.Paused = false
	pr.Active = false
}

func (pr *Progress) becomeProbe() {
	if pr.State == raftpb.PROGRESS_STATE_SNAPSHOT { // snapshot was sent
		pIdx := pr.PendingSnapshotIndex
		pr.resetState(raftpb.PROGRESS_STATE_PROBE)
		pr.NextIndex = maxUint64(pr.MatchIndex+1, pIdx+1)
		return
	}
	pr.resetState(raftpb.PROGRESS_STATE_PROBE)
	pr.NextIndex = pr.MatchIndex + 1 // probe next index
}

func (pr *Progress) becomeReplicate() {
	pr.resetState(raftpb.PROGRESS_STATE_REPLICATE)
	pr.NextIndex = pr.MatchIndex + 1 // probe next index
}

func (pr *Progress) pause() {
	pr.Paused = true
}

func (pr *Progress) resume() {
	pr.Paused = false
}

func (pr *Progress) isPaused() bool {
	switch pr.State {
	case raftpb.PROGRESS_STATE_PROBE:
		return pr.Paused
	case raftpb.PROGRESS_STATE_REPLICATE:
		return pr.inflights.full()
	case raftpb.PROGRESS_STATE_SNAPSHOT:
		return true
	default:
		panic("unexpected pr.State")
	}
}

func (pr *Progress) becomeSnapshot(snapshotIndex uint64) {
	pr.resetState(raftpb.PROGRESS_STATE_SNAPSHOT)
	pr.PendingSnapshotIndex = snapshotIndex
}

func (pr *Progress) optimisticUpdate(msgLogIndex uint64) {
	pr.NextIndex = msgLogIndex + 1
}

// maybeUpdate returns false if the given index comes from an
// outdated message.
//
// (etcd raft.Progress.maybeUpdate)
func (pr *Progress) maybeUpdate(msgLogIndex uint64) bool {
	upToDate := false
	if pr.MatchIndex < msgLogIndex { // update MatchIndex
		pr.MatchIndex = msgLogIndex
		upToDate = true
		pr.resume()
	}

	if pr.NextIndex <= msgLogIndex { // update NextIndex
		pr.NextIndex = msgLogIndex + 1
	}

	return upToDate
}

// maybeDecrease returns true if the rejecting message's log index
// comes from an outdated message. Otherwise, it decreases the next
// index in the follower's progress, and returns true.
//
// (etcd raft.Progress.maybeDecrTo)
func (pr *Progress) maybeDecrease(rejectLogIndex, rejectHint uint64) bool {
	if pr.State == raftpb.PROGRESS_STATE_REPLICATE {
		if rejectLogIndex <= pr.MatchIndex {
			return false
		}

		pr.NextIndex = pr.MatchIndex + 1
		return true
	}

	if pr.NextIndex-1 != rejectLogIndex {
		return false
	}

	pr.NextIndex = minUint64(rejectLogIndex, rejectHint+1)
	if pr.NextIndex < 1 {
		pr.NextIndex = 1
	}

	pr.resume()
	return true
}

// needSnapshotAbort returns true if it needs to stop sending snapshot to the
// follower.
func (pr *Progress) needSnapshotAbort() bool {
	return pr.State == raftpb.PROGRESS_STATE_SNAPSHOT && pr.MatchIndex >= pr.PendingSnapshotIndex
}

func (pr *Progress) snapshotFailed() {
	pr.PendingSnapshotIndex = 0 // reset because it failed
}

func (pr *Progress) String() string {
	return fmt.Sprintf("[state=%q | match index=%d | next index=%d | paused(waiting)=%v | pending Snapshot index=%d]",
		pr.State,
		pr.MatchIndex,
		pr.NextIndex,
		pr.isPaused(),
		pr.PendingSnapshotIndex,
	)
}
