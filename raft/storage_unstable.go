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
		// (X) return su.entries[len(su.entries)-1].Index, true
		// OR
		return su.indexOffset + uint64(len(su.entries)) - 1, true

	case su.incomingSnapshot != nil: // no unstable entries
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
		// index offset = 0 + 10 = 10
		//
		// persistedEntriesAt(index=10) = entries[10-10+1:] = entries[1:]
		//                    = [11, 12]
		//
		// [10] is now considered persisted to stable storage
		// Now index offset = 10 + 1 = 11
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
func (su *storageUnstable) restoreIncomingSnapshot(snap raftpb.Snapshot) {
	su.indexOffset = snap.Metadata.Index + 1
	su.entries = nil
	su.incomingSnapshot = &snap
}

func (su *storageUnstable) truncateAndAppend(entries []raftpb.Entry) {
	firstIndexInEntriesToAppend := entries[0].Index
	switch {
	case firstIndexInEntriesToAppend == su.indexOffset+uint64(len(su.entries)):
		// entries in unstable = [10, 11, 12]
		// index offset        = 10
		//
		// entries to append   = [13, 14]
		//
		// first index in entries to append = 13
		// index offset + len(unstable entries) = 10 + 3 = 13
		//
		// first index in entries to append == index offset + len(unstable entries)
		//                               13 == 13
		// ➝ just append
		//
		su.entries = append(su.entries, entries...)

	case firstIndexInEntriesToAppend <= su.indexOffset:
		// entries in unstable = [10, 11, 12]
		// index offset        = 10
		//
		// entries to append   = [9, 10, 11, 12, 13]
		//
		// first index in entries to append = 9
		// index offset + len(unstable entries) = 10 + 3 = 13
		//
		// first index in entries to append != index offset + len(unstable entries)
		// first index in entries to append <= index offset
		//                                9 <= 10
		// ➝ need to replace unstable entries from "first index in entries to append"
		//
		// ➝ index offset        = first index in entries to append = 9
		// ➝ entries in unstable = entries to append                = [9, 10, 11, 12, 13]
		//
		raftLogger.Infof("replacing unstable entries from index %d", firstIndexInEntriesToAppend)
		su.indexOffset = firstIndexInEntriesToAppend
		su.entries = entries

	default:
		// entries in unstable = [10, 11, 12]
		// index offset        = 10
		//
		// entries to append   = [11, 12, 13]
		//
		// first index in entries to append = 11
		// index offset + len(unstable entries) = 10 + 3 = 13
		//
		// first index in entries to append != index offset + len(unstable entries)
		// first index in entries to append > index offset
		//                               11 > 10
		// ➝ need to truncate unstable entries before appending
		//
		// ➝ entries in unstable = entries to append [index offset - index offset : first index in entries to append - index offset]
		//                       = entries[10-10:11-10] = entries[:1] = [10]
		//
		// ➝ now appends to the truncated unstable entries
		// ➝ [10] + [11, 12, 13] = [10, 11, 12, 13]
		//
		raftLogger.Infof("truncating unstable entries with unstable.entries[%d, %d)", su.indexOffset, firstIndexInEntriesToAppend)
		oldN := len(su.entries)
		sl := su.slice(su.indexOffset, firstIndexInEntriesToAppend)
		tmps := make([]raftpb.Entry, len(sl))
		copy(tmps, sl)
		su.entries = tmps

		raftLogger.Infof("truncated %d entries, now appending %d entries", oldN-len(su.entries), len(entries))
		su.entries = append(su.entries, entries...)
	}
}

func (su *storageUnstable) checkSliceBoundary(startIdx, endIdx uint64) {
	if startIdx > endIdx {
		raftLogger.Panicf("invalid unstable indexes [start index = %d | end index = %d]", startIdx, endIdx)
	}

	lastIndex := su.indexOffset + uint64(len(su.entries))
	if startIdx < su.indexOffset || endIdx > lastIndex {
		raftLogger.Panicf("unstable.entries[%d,%d) is out of bound [index offset = %d | last index = %d]", startIdx, endIdx, su.indexOffset, lastIndex)
	}
}

func (su *storageUnstable) slice(startIdx, endIdx uint64) []raftpb.Entry {
	su.checkSliceBoundary(startIdx, endIdx)
	return su.entries[startIdx-su.indexOffset : endIdx-su.indexOffset]
}
