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
	//
	// Snapshot represents current in-memory state of the server.
	// It is stored in stable storage before messages are sent.
	//
	// To prevent logs from growing unbounded, Raft servers compact their
	// logs when the quorum of cluster is available. But when the unavailable
	// minority of servers become available, they need to catch up, by
	// receiving the snapshots from leader, because those missing log entries
	// are possibly gone forever(compacted). So leader sometimes sends snapshots
	// to each other across the network. Write-ahead log approach continuously
	// snapshots to the disk.
	//
	// When the leader decides to snapshot, it first logs a special start
	// entry. For slow followers, the leader will have to send an AppendEntries
	// RPC to the follower with an entry that the leader has already discarded
	// (compacted). In this case, the follower should just discard its entire
	// state. The leader will send the follower the log entries beginning with
	// the start entry.
	//
	// (etcd raft.Storage.Snapshot)
	Snapshot() (raftpb.Snapshot, error)
}
