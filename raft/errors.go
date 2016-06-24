package raft

import "errors"

var (
	// ErrStopped is returned when a Node has been stopped.
	ErrStopped = errors.New("stoppped")

	// ErrUnavailable is returned when creating snapshot and
	// the requested log entries aren't available.
	ErrUnavailable = errors.New("requested entry at index is unavailable")

	// ErrCompacted indicates that requested index is unavailable
	// because it predates the last snapshot.
	ErrCompacted = errors.New("requested index is unavailable (already compacted)")

	// ErrSnapOutOfDate is returned when creating snapshot and the requested
	// index is older than the existing snapshot.
	ErrSnapOutOfDate = errors.New("requested index is older than the existing snapshot")

	// ErrSnapshotTemporarilyUnavailable is returned when the required
	// snapshot is temporarily unavailable.
	ErrSnapshotTemporarilyUnavailable = errors.New("snapshot is temporarily available")
)
