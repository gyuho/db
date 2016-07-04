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

	// entries[idx]'s raft log index == idx + su.indexOffset
	entries []raftpb.Entry
}

// maybeFirstIndex returns the index of first available entry in incoming snapshot.
//
// (etcd raft.unstable.maybeFirstIndex)
func (su *storageUnstable) maybeFirstIndex() (uint64, bool) {
	if su.incomingSnapshot != nil {
		// "+ 1" because first entry is for compacted entry
		// kept for matching purposes.
		//
		// StorageStable.snapshotEntries[idx]'s raft log index == idx + snapshot.Metadata.Index
		// And StorageStable.snapshotEntries[0] is a dummy entry.
		return su.incomingSnapshot.Metadata.Index + 1, true
	}
	return 0, false
}

// maybeLastIndex returns the last index of unstable entries or snapshot.
//
// (etcd raft.unstable.maybeLastIndex)
func (su *storageUnstable) maybeLastIndex() (uint64, bool) {
	switch {
	case len(su.entries) > 0:
		// return su.entries[len(su.entries)-1].Index, true
		// OR
		return su.indexOffset + uint64(len(su.entries)) - 1, true

	case su.incomingSnapshot != nil: // no unstable entries
		return su.incomingSnapshot.Metadata.Index, true

	default:
		return 0, false
	}
}

// maybeTerm returns the term of the entry with log index idx, if any.
//
// (etcd raft.unstable.maybeTerm)
func (su *storageUnstable) maybeTerm(idx uint64) (uint64, bool) {
	if idx < su.indexOffset {
		if su.incomingSnapshot == nil {
			return 0, false
		}
		if su.incomingSnapshot.Metadata.Index == idx { // no matching unstable entries
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

// persistedEntriesAt updates unstable entries and indexes after persisting
// those unstable entries to stable storage.
//
// (etcd raft.unstable.stableTo)
func (su *storageUnstable) persistedEntriesAt(index, term uint64) {
	tm, ok := su.maybeTerm(index)
	if !ok {
		return
	}

	// only update unstable entries if term
	// is matched with an unstable entry
	if tm == term && index >= su.indexOffset {
		// entries      = [10, 11, 12]
		// indexOffset = 0 + 10 = 10
		//
		// persistedEntriesAt(index=10) = entries[10-10+1:] = entries[1:]
		//                    = [11, 12]
		//
		// [10] is now considered persisted to stable storage
		// Now indexOffset = 10 + 1 = 11
		//
		su.entries = su.entries[index-su.indexOffset+1:]
		su.indexOffset = index + 1
	}
}

// persistedSnapshotAt updates snapshot metadata after processing the incoming snapshot.
//
// (etcd raft.unstable.stableSnapTo)
func (su *storageUnstable) persistedSnapshotAt(index uint64) {
	if su.incomingSnapshot != nil && su.incomingSnapshot.Metadata.Index == index {
		su.incomingSnapshot = nil
	}
}

// restoreIncomingSnapshot sets unstable storage with incoming snapshot.
//
// (etcd raft.unstable.restore)
func (su *storageUnstable) restoreIncomingSnapshot(snap raftpb.Snapshot) {
	su.indexOffset = snap.Metadata.Index + 1
	su.entries = nil
	su.incomingSnapshot = &snap
}

// truncateAndAppend appends new entries with truncation if needed.
//
// (etcd raft.unstable.truncateAndAppend)
func (su *storageUnstable) truncateAndAppend(entries ...raftpb.Entry) {
	if len(entries) == 0 {
		return
	}

	firstIndexInEntriesToAppend := entries[0].Index
	switch {
	case firstIndexInEntriesToAppend == su.indexOffset+uint64(len(su.entries)):
		// su.entries   = [10, 11, 12]
		// indexOffset  = 10
		//
		// entries to append = [13, 14]
		//
		// firstIndexInEntriesToAppend = 13
		// indexOffset + len(unstable entries) = 10 + 3 = 13
		//
		// firstIndexInEntriesToAppend == indexOffset + len(unstable entries)
		//                          13 == 13
		// ➝ just append
		//
		su.entries = append(su.entries, entries...)

	case firstIndexInEntriesToAppend <= su.indexOffset:
		// su.entries   = [10, 11, 12]
		// indexOffset  = 10
		//
		// entries to append = [9, 10, 11, 12, 13]
		//
		// firstIndexInEntriesToAppend = 9
		// indexOffset + len(unstable entries) = 10 + 3 = 13
		//
		// firstIndexInEntriesToAppend != indexOffset + len(unstable entries)
		// firstIndexInEntriesToAppend <= indexOffset
		//                           9 <= 10
		// ➝ need to replace unstable entries from "firstIndexInEntriesToAppend"
		//
		// ➝ indexOffset = firstIndexInEntriesToAppend = 9
		// ➝ su.entries  = entries to append           = [9, 10, 11, 12, 13]
		//
		raftLogger.Infof("replacing unstable entries from index '%d'", firstIndexInEntriesToAppend)
		su.indexOffset = firstIndexInEntriesToAppend
		su.entries = entries

	default:
		// su.entries     = [10, 11, 12]
		// su.indexOffset = 10
		//
		// entries to append = [11, 12, 13]
		//
		// firstIndexInEntriesToAppend = 11
		// su.indexOffset + len(unstable entries) = 10 + 3 = 13
		//
		// firstIndexInEntriesToAppend != su.indexOffset + len(unstable entries)
		// firstIndexInEntriesToAppend  > su.indexOffset
		//                          11 > 10
		// ➝ need to truncate unstable entries before appending
		//
		// ➝ su.entries = entries to append [su.indexOffset - su.indexOffset : firstIndexInEntriesToAppend - su.indexOffset]
		//              = su.entries[10 - 10 : 11 - 10]
		//              = su.entries[:1]
		//              = [10]
		//
		raftLogger.Infof("truncating unstable entries in [start index='%d', end='%d')", su.indexOffset, firstIndexInEntriesToAppend)
		oldN := len(su.entries)
		sl := su.slice(su.indexOffset, firstIndexInEntriesToAppend)
		tmps := make([]raftpb.Entry, len(sl))
		copy(tmps, sl)
		su.entries = tmps

		// ➝ now append new entries to the truncated unstable entries
		// ➝ [10] + [11, 12, 13] = [10, 11, 12, 13]
		//
		raftLogger.Infof("truncated %d entries, now appending %d entries", oldN-len(su.entries), len(entries))
		su.entries = append(su.entries, entries...)
	}
}

// mustCheckOutOfBounds ensures that:
//
//   su.indexOffset <= startIndex <= endIndex <= su.lastIndex
//
// (etcd raft.unstable.mustCheckOutOfBounds)
func (su *storageUnstable) mustCheckOutOfBounds(startIndex, endIndex uint64) {
	if startIndex > endIndex {
		raftLogger.Panicf("invalid unstable indexes [start index = %d | end index = %d]", startIndex, endIndex)
	}

	lastIndex := su.indexOffset + uint64(len(su.entries))
	if startIndex < su.indexOffset || endIndex > lastIndex {
		raftLogger.Panicf("unstable.entries[%d,%d) is out of bound [indexOffset = %d | last index = %d]", startIndex, endIndex, su.indexOffset, lastIndex)
	}
}

// slice returns entries[startIndex, endIndex).
func (su *storageUnstable) slice(startIndex, endIndex uint64) []raftpb.Entry {
	su.mustCheckOutOfBounds(startIndex, endIndex)
	return su.entries[startIndex-su.indexOffset : endIndex-su.indexOffset]
}
