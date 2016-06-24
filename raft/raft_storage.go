package raft

import "github.com/gyuho/dba/raftpb"

// Storage defines storage interface that retrieves log entries from storage.
//
// (etcd raft.Storage)
type Storage interface {
	// LoadInitialState returns the saved HardState
	// and ConfigState.
	//
	// (etcd raft.Storage.InitialState)
	LoadInitialState() (raftpb.HardState, raftpb.ConfigState, error)

	// Entries returns the slice of log entries in [idx1, idx2).
	// maxNum limits the number of log entries returned,
	// and Entries returns at least one entry if any.
	//
	// (etcd raft.Storage.Entries)
	Entries(idx1, idx2, maxNum uint64) ([]raftpb.Entry, error)

	// Term returns the term of the entry index, which must be in the range
	// [FirstLogIndex - 1, LastLogIndex]. The term of the entry before
	// FirstLogIndex is retained for matching purposes, even if the rest of
	// the entries in the term may not be available.
	Term(idx uint64) (uint64, error)

	// LastLogIndex returns the index of the last log entry.
	//
	// (etcd raft.Storage.LastIndex)
	LastLogIndex() (uint64, error)

	// FirstLogIndex returns the index of the first-available
	// log entry, because older entries have been incorporated
	// into the latest snapshot.
	//
	// (etcd raft.Storage.FirstIndex
	FirstLogIndex() (uint64, error)

	// Snapshot returns the most recent snapshot.
	// If the snapshot is temporarily unavailable, the state machine
	// knows that Storage needs more time to prepare snapshot.
	Snapshot() (raftpb.Snapshot, error)
}
