package raft

import "errors"

var (
	// ErrStopped is returned when a Node has been stopped.
	ErrStopped = errors.New("raft: stoppped")

	// ErrUnavailable is returned when creating snapshot and
	// the requested log entries aren't available.
	ErrUnavailable = errors.New("raft: requested entry at index is unavailable")

	// ErrCompacted indicates that requested index is unavailable
	// because it predates the last snapshot.
	ErrCompacted = errors.New("raft: requested index is unavailable (already compacted)")

	// ErrSnapOutOfDate is returned when creating snapshot and the requested
	// index is older than the existing snapshot.
	ErrSnapOutOfDate = errors.New("raft: requested index is older than the existing snapshot")

	// ErrSnapshotTemporarilyUnavailable is returned when the required
	// snapshot is temporarily unavailable.
	ErrSnapshotTemporarilyUnavailable = errors.New("raft: snapshot is temporarily available")
)
