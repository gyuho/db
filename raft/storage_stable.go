package raft

import "github.com/gyuho/db/raft/raftpb"

// StorageStable defines storage interface that retrieves log entries from storage.
//
// (etcd raft.Storage)
type StorageStable interface {
	// GetState returns the saved HardState and ConfigState.
	//
	// (etcd raft.Storage.InitialState)
	GetState() (raftpb.HardState, raftpb.ConfigState, error)

	// FirstIndex returns the index of the first-available log entry.
	// Older entries have been incorporated into the latest snapshot.
	//
	// (etcd raft.Storage.FirstIndex)
	FirstIndex() (uint64, error)

	// LastIndex returns the index of the last log entry in storage.
	//
	// (etcd raft.Storage.LastIndex)
	LastIndex() (uint64, error)

	// Term returns the term of the entry index, which must be in the range
	// [FirstIndex - 1, LastIndex].
	//
	// The term of the entry before FirstIndex is retained for matching purposes,
	// even if the rest of the entries in the term may not be available.
	//
	// (etcd raft.Storage.Term)
	Term(index uint64) (uint64, error)

	// Entries returns the slice of log entries in [startIndex, endIndex).
	// limitSize limits the total size of log entries to return.
	// It returns at least one entry if any.
	//
	// (etcd raft.Storage.Entries)
	Entries(startIndex, endIndex, limitSize uint64) ([]raftpb.Entry, error)

	// Snapshot returns the most recent snapshot.
	// If the snapshot is temporarily unavailable, the state machine
	// knows that Storage needs more time to prepare snapshot.
	Snapshot() (raftpb.Snapshot, error)
}
