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

// maybeTerm returns the term of the entry with log index idx, if any.
func (su *storageUnstable) maybeTerm(idx uint64) (uint64, bool) {
	if idx < su.indexOffset {
		if su.incomingSnapshot == nil {
			return 0, false
		}
		if su.incomingSnapshot.Metadata.Index == idx {
			return su.incomingSnapshot.Metadata.Term, true
		}
		return 0, false
	}

	lasti, ok := su.maybeLastIndex()
	if !ok {
		return 0, false
	}

	if idx > lasti {
		return 0, false
	}

	return su.entries[idx-su.indexOffset].Term, true
}

func (su *storageUnstable) stableTo(index, term uint64) {
	tm, ok := su.maybeTerm(index)
	if !ok {
		return
	}

	// only update unstable entries if term
	// is matched with an unstable entry
	if tm == term && index >= su.indexOffset {
		// entries      = [10, 11, 12]
		// index offset = 0 + 10 = 10
		//
		// stableTo(index=10) = entries[10-10+1:] = entries[1:]
		//                    = [11, 12]
		//
		// [10] is now considered persisted to stable storage
		//
		su.entries = su.entries[index-su.indexOffset+1:]
		su.indexOffset = index + 1
	}
}
