package raft

import "github.com/gyuho/db/raft/raftpb"

// storageUnstable stores unstable entries that have not yet
// been written to the Storage.
//
// (etcd raft.unstable)
type storageUnstable struct {
	incomingSnapshot *raftpb.Snapshot

	// indexOffset may be smaller than the actual higest log
	// position in Storage, which means the next write to storage
	// might need truncate the logs before persisting these unstable
	// log entries.
	indexOffset uint64

	// entries[idx]'s raft log index == idx + indexOffset
	entries []raftpb.Entry
}

// maybeFirstIndex returns the index of first available entry in snapshot.
func (su *storageUnstable) maybeFirstIndex() (uint64, bool) {
	if su.incomingSnapshot != nil {
		// ??? "+ 1" to
		return su.incomingSnapshot.Metadata.Index + 1, true
	}
	return 0, false
}

// maybeLastIndex returns the last index of unstable entries or snapshot.
func (su *storageUnstable) maybeLastIndex() (uint64, bool) {
	switch {
	case len(su.entries) > 0:
		return su.entries[len(su.entries)-1].Index, true
		// OR
		// return su.indexOffset + uint64(len(su.entries)) - 1

	case su.incomingSnapshot != nil: // no unstable entries
		// ??? no "+1" because
		return su.incomingSnapshot.Metadata.Index, true

	default:
		return 0, false
	}
}
