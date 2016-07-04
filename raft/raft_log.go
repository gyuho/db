package raft

import (
	"fmt"
	"math"

	"github.com/gyuho/db/raft/raftpb"
)

// raftLog represents raft log and its storage.
//
// (etcd raft.raftLog)
type raftLog struct {
	// storageStable contains all stable entries since the last snapshot.
	storageStable StorageStable

	// storageUnstable contains all unstable entries and snapshot to store into stableStorage.
	// No need to define in pointer because 'raftLog' will be passed with pointer.
	storageUnstable storageUnstable

	// committedIndex is the higest log position that is known to be stored in
	// the stable storage "on a quorum of nodes".
	committedIndex uint64

	// appliedIndex is the highest log position that the application has been
	// instructed to apply to its state machine.
	// Must: appliedIndex <= committedIndex
	appliedIndex uint64
}

// newRaftLog returns a new raftLog with specified stable storage.
//
// (etcd raft.raftLog.newLog)
func newRaftLog(storageStable StorageStable) *raftLog {
	if storageStable == nil {
		raftLogger.Panic("stable storage must not be nil")
	}

	rg := &raftLog{
		storageStable: storageStable,
	}

	firstIndex, err := storageStable.FirstIndex()
	if err != nil {
		raftLogger.Panicf("stable storage first index is not available (%v)", err)
	}

	lastIndex, err := storageStable.LastIndex()
	if err != nil {
		raftLogger.Panicf("stable storage last index is not available (%v)", err)
	}

	rg.storageUnstable.incomingSnapshot = nil
	rg.storageUnstable.indexOffset = lastIndex + 1
	rg.storageUnstable.entries = nil

	// index of last compaction
	rg.committedIndex = firstIndex - 1
	rg.appliedIndex = firstIndex - 1

	return rg
}

func (rg *raftLog) String() string {
	return fmt.Sprintf("[commit index = %d | applied index = %d | unstable.indexOffset = %d | len(unstanble.entries) = %d]",
		rg.committedIndex, rg.appliedIndex,
		rg.storageUnstable.indexOffset, len(rg.storageUnstable.entries),
	)
}

// firstIndex gets the first index from unstable storage first.
// If it's not available, try to get the first index in stable storage.
//
// (etcd raft.raftLog.firstIndex)
func (rg *raftLog) firstIndex() uint64 {
	if index, ok := rg.storageUnstable.maybeFirstIndex(); ok {
		return index
	}

	index, err := rg.storageStable.FirstIndex()
	if err != nil {
		raftLogger.Panicf("raftLog.storageStable.FirstIndex error (%v)", err)
	}

	return index
}

// lastIndex gets the last index from unstable storage first.
// If it's not available, try to get the last index in stable storage.
//
// (etcd raft.raftLog.lastIndex)
func (rg *raftLog) lastIndex() uint64 {
	if index, ok := rg.storageUnstable.maybeLastIndex(); ok {
		return index
	}

	index, err := rg.storageStable.LastIndex()
	if err != nil {
		raftLogger.Panicf("raftLog.storageStable.LastIndex error (%v)", err)
	}

	return index
}

// term gets the term of specified index from unstable storage first.
// If it's not available, try to get the term in stable storage.
//
// (etcd raft.raftLog.term)
func (rg *raftLog) term(index uint64) (uint64, error) {
	dummyIndex := rg.firstIndex() - 1
	if index < dummyIndex || rg.lastIndex() < index {
		raftLogger.Warningf("out-of-range index '%d' [dummy index=%d, last index=%d]", index, dummyIndex, rg.lastIndex())
		return 0, nil
	}

	if tm, ok := rg.storageUnstable.maybeTerm(index); ok {
		return tm, nil
	}

	tm, err := rg.storageStable.Term(index)
	switch err {
	case nil:
		return tm, nil
	case ErrCompacted:
		return 0, err
	default:
		panic(err)
	}
}

// lastTerm returns the term of raftLog's last log entry.
//
// (etcd raft.raftLog.lastTerm)
func (rg *raftLog) lastTerm() uint64 {
	tm, err := rg.term(rg.lastIndex())
	if err != nil {
		raftLogger.Panicf("raftLog.term(rg.lastIndex) error (%v)", err)
	}
	return tm
}

// matchTerm returns true if log index does actually has the 'term'.
//
// (etcd raft.raftLog.matchTerm)
func (rg *raftLog) matchTerm(index, term uint64) bool {
	tm, err := rg.term(index)
	if err != nil {
		return false
	}
	return tm == term
}

// mustCheckOutOfBounds ensures that:
//
//   rg.firstIndex() <= startIndex <= endIndex <= rg.firstIndex() + len(rg.entries)
//
// (etcd raft.raftLog.mustCheckOutOfBounds)
func (rg *raftLog) mustCheckOutOfBounds(startIndex, endIndex uint64) error {
	if startIndex > endIndex {
		raftLogger.Panicf("invalid raft log indexes [start index=%d | end index=%d]", startIndex, endIndex)
	}

	firstIndex := rg.firstIndex()
	if firstIndex > startIndex {
		return ErrCompacted
	}

	entryN := rg.lastIndex() - firstIndex + 1
	if endIndex > firstIndex+entryN {
		raftLogger.Panicf("entries[%d, %d) is out of bound [first index=%d | last index=%d]", startIndex, endIndex, firstIndex, rg.lastIndex())
	}

	return nil
}

// slice returns the entries[startIndex, endIndex) with limit size.
//
// (etcd raft.raftLog.slice)
func (rg *raftLog) slice(startIndex, endIndex, limitSize uint64) ([]raftpb.Entry, error) {
	if err := rg.mustCheckOutOfBounds(startIndex, endIndex); err != nil {
		return nil, err
	}

	if startIndex == endIndex {
		return nil, nil
	}

	var entries []raftpb.Entry

	// For example,
	//
	// entries in stable storage        = [147, 148, 149]
	// entries in unstable storage      = [150, 151]
	// index offset in unstable storage = 150
	//
	// slice(startIndex=148, endIndex=152)
	//
	// ➝ startIndex < index offset in unstable storage
	// ➝        148 < 150
	//
	// ➝ stableStorage.Entries( startIndex , min( endIndex , index offset in unstable storage ) )
	// ➝ stableStorage.Entries( 148 , min(152,150) )
	// ➝ stableStorage.Entries( 148 , 150 )
	//   == stableStorage.Entries[startIndex, endIndex)
	// ➝ [148, 149]
	//
	if startIndex < rg.storageUnstable.indexOffset { // try stable storage entries
		stableEntries, err := rg.storageStable.Entries(startIndex, minUint64(endIndex, rg.storageUnstable.indexOffset), limitSize)
		if err != nil {
			switch err {
			case ErrCompacted:
				return nil, err
			case ErrUnavailable:
				raftLogger.Panicf("entries[%d,%d) is unavailable from stable storage", startIndex, minUint64(endIndex, rg.storageUnstable.indexOffset))
			default:
				raftLogger.Panicf("entries[%d,%d) is unavailable from stable storage (%v)", startIndex, minUint64(endIndex, rg.storageUnstable.indexOffset), err)
			}
		}

		// stableEntriesN = len([147, 148, 149]) = 3
		// expectedN      = min(index offset in unstable storage, endIndex) - startIndex
		//                = min(150,152) - 147 = 150 - 147 = 3
		//
		// stableEntriesN >= expectedN
		// ➝ need to check limits
		//
		var (
			stableEntriesN = uint64(len(stableEntries))
			expectedN      = minUint64(rg.storageUnstable.indexOffset, endIndex) - startIndex
		)
		if stableEntriesN < expectedN { // no need to check limits
			return stableEntries, nil
		}

		// For example,
		//
		// entries in stable storage        = [147, 148, 149]
		// entries in unstable storage      = [151, 152]
		// index offset in unstable storage = 151
		//
		// slice(startIndex=147, endIndex=153)
		//
		// ➝ startIndex < index offset in unstable storage
		// ➝        147 < 151
		//
		// ➝ stableStorage.Entries( startIndex , min( endIndex , index offset in unstable storage ) )
		// ➝ stableStorage.Entries( 147 , min(153,151) )
		// ➝ stableStorage.Entries( 147 , 151 )
		//
		// endIndex > ms.lastIndex()+1 == 149 + 1 == 150
		//      153 > 150
		// ➝ out of bound panic!

		// For example,
		//
		// entries in stable storage        = [148, 149]
		// entries in unstable storage      = [150, 151]
		// index offset in unstable storage = 150
		//
		// slice(startIndex=148, endIndex=152)
		//
		// ➝ startIndex < index offset in unstable storage
		// ➝        148 < 150
		//
		// ➝ stableStorage.Entries( startIndex , min( endIndex , index offset in unstable storage ) )
		// ➝ stableStorage.Entries( 148 , min (152,150) )
		// ➝ stableStorage.Entries( 148 , 150 )
		// ➝ stableStorage.Entries[startIndex, endIndex)
		// ➝ [148, 149]
		//
		// stableEntriesN = len([148, 149]) = 2
		// expectedN      = min(index offset in unstable storage, endIndex) - startIndex
		//                = min(150,152) - 148 = 150 - 148 = 2
		//
		// stableEntriesN >= expectedN
		// ➝ need to check limits

		entries = stableEntries
	}

	// For example,
	//
	// entries in stable storage        = [147, 148, 149]
	// entries in unstable storage      = [150, 151]
	// index offset in unstable storage = 150
	//
	// slice(startIndex=148, endIndex=152)
	//
	// ➝ startIndex < index offset in unstable storage
	// ➝        148 < 150
	//
	// ➝ stableStorage.Entries( startIndex , min( endIndex , index offset in unstable storage ) )
	// ➝ stableStorage.Entries( 148 , min(152,150) )
	// ➝ stableStorage.Entries( 148 , 150 )
	//   == stableStorage.Entries[startIndex, endIndex)
	// ➝ [148, 149]
	//
	//
	// ➝ endIndex > index offset in unstable storage
	// ➝      151 > 150
	//
	// ➝ ents = unstableEntries.slice( max( startIndex , index offset in unstable storage ) , endIndex )
	//        = unstableEntries.slice( max(150,150), 152 )
	//        = unstableEntries.slice( 150, 152 )
	//        = [150, 151]
	//
	// ➝ return [148, 149] + [150, 151]
	//
	if endIndex > rg.storageUnstable.indexOffset { // try unstable storage entries
		unstableEntries := rg.storageUnstable.slice(maxUint64(startIndex, rg.storageUnstable.indexOffset), endIndex)
		if len(entries) > 0 { // there are entries from stable storage
			// (X) ??? race condition in etcd integration test!
			// entries = append(entries, unstableEntries...)

			// entries = append([]raftpb.Entry{}, entries...)
			//
			// OR
			//
			tmps := make([]raftpb.Entry, len(entries))
			copy(tmps, entries)
			entries = tmps

			entries = append(entries, unstableEntries...)
		} else {
			entries = unstableEntries
		}
	}

	return limitEntries(entries, limitSize), nil
}

// entries returns the entries[startIndex,rg.endIndex+1) with size limit.
//
// (etcd raft.raftLog.entries)
func (rg *raftLog) entries(startIndex, limitSize uint64) ([]raftpb.Entry, error) {
	if startIndex > rg.lastIndex() {
		return nil, nil
	}
	return rg.slice(startIndex, rg.lastIndex()+1, limitSize)
}

// unstableEntries returns all unstable entries in raft log.
//
// (etcd raft.raftLog.unstableEntries)
func (rg *raftLog) unstableEntries() []raftpb.Entry {
	if len(rg.storageUnstable.entries) == 0 {
		return nil
	}
	return rg.storageUnstable.entries
}

// allEntries returns all entries in raft log, except the first dummy entry.
//
// (etcd raft.raftLog.allEntries)
func (rg *raftLog) allEntries() []raftpb.Entry {
	entries, err := rg.entries(rg.firstIndex(), math.MaxUint64)
	if err != nil {
		switch err {
		case ErrCompacted: // try again in case there was a racing compaction
			return rg.allEntries()
		default:
			raftLogger.Panicf("raftLog.allEntries error (%v)", err)
		}
	}
	return entries
}

// snapshot returns the snapshot of current raft log.
// It first tries to get the snapshot in unstable storage.
// If not available, try the one in stable storage.
//
// (etcd raft.raftLog.snapshot)
func (rg *raftLog) snapshot() (raftpb.Snapshot, error) {
	if rg.storageUnstable.incomingSnapshot != nil { // try the unstable storage first
		return *rg.storageUnstable.incomingSnapshot, nil
	}

	return rg.storageStable.Snapshot()
}

// restoreIncomingSnapshot sets unstable storage with incoming snapshot.
//
// (etcd raft.raftLog.restore)
func (rg *raftLog) restoreIncomingSnapshot(snap raftpb.Snapshot) {
	raftLogger.Infof("raft log %q is restroing the incoming snapshot [snapshot index=%d | snapshot term=%d]", rg, snap.Metadata.Index, snap.Metadata.Term)
	rg.committedIndex = snap.Metadata.Index
	rg.storageUnstable.restoreIncomingSnapshot(snap)
}

// hasNextEntriesToApply returns true if there are entries
// availablefor execution.
//
// (etcd raft.raftLog.hasNextEnts)
func (rg *raftLog) hasNextEntriesToApply() (uint64, bool) {
	maxStart := maxUint64(rg.appliedIndex+1, rg.firstIndex())
	return maxStart, rg.committedIndex >= maxStart
}

// nextEntriesToApply returns all the available entries ready for applying(execution).
//
// (etcd raft.raftLog.nextEnts)
func (rg *raftLog) nextEntriesToApply() []raftpb.Entry {
	maxStart, ok := rg.hasNextEntriesToApply()
	if !ok {
		return nil
	}

	entries, err := rg.slice(maxStart, rg.committedIndex+1, math.MaxUint64)
	if err != nil {
		raftLogger.Panicf("nextEntriesToApply error (%v)", err)
	}
	return entries
}

// zeroTermOnErrCompacted returns 0 if specified error is ErrCompacted.
//
// (etcd raft.raftLog.zeroTermOnErrCompacted)
func (rg *raftLog) zeroTermOnErrCompacted(term uint64, err error) uint64 {
	switch err {
	case nil:
		return term

	case ErrCompacted:
		return 0

	default:
		raftLogger.Panicf("unexpected error (%v)", err)
		return 0
	}
}

// isUpToDate returns true if the given (lastIndex, term) log is more
// up-to-date than the last entry in the existing logs.
//
// (etcd raft.raftLog.isUpToDate)
func (rg *raftLog) isUpToDate(lastIndex, term uint64) bool {
	return term > rg.lastTerm() || (term == rg.lastTerm() && lastIndex >= rg.lastIndex())
}

// persistedEntriesAt updates unstable entries and indexes after persisting
// those unstable entries to stable storage.
//
// (etcd raft.raftLog.stableTo)
func (rg *raftLog) persistedEntriesAt(index, term uint64) {
	rg.storageUnstable.persistedEntriesAt(index, term)
}

// persistedSnapshotAt updates snapshot metadata after processing the incoming snapshot.
//
// (etcd raft.raftLog.stableSnapTo)
func (rg *raftLog) persistedSnapshotAt(index uint64) {
	rg.storageUnstable.persistedSnapshotAt(index)
}

// commitTo updates raftLog's committedIndex.
//
// (etcd raft.raftLog.commitTo)
func (rg *raftLog) commitTo(committedIndex uint64) {
	// to never decrease commit index
	if rg.committedIndex < committedIndex {
		if rg.lastIndex() < committedIndex {
			raftLogger.Panicf("got wrong commit index '%d', smaller than last index '%d' (possible log corruption, truncation, lost)",
				committedIndex, rg.lastIndex())
		}
		rg.committedIndex = committedIndex
	}
}

// appliedTo updates raftLog's appliedIndex.
//
// (etcd raft.raftLog.appliedTo)
func (rg *raftLog) appliedTo(appliedIndex uint64) {
	if appliedIndex == 0 {
		return
	}

	// MUST "rg.committedIndex >= appliedIndex"
	if rg.committedIndex < appliedIndex || appliedIndex < rg.appliedIndex {
		raftLogger.Panicf("got wrong applied index %d [commit index=%d | previous applied index=%d]",
			appliedIndex, rg.committedIndex, rg.appliedIndex)
	}

	rg.appliedIndex = appliedIndex
}

// maybeCommit returns true if commitTo operation was successful
// with given index and term.
//
// (etcd raft.raftLog.maybeCommit)
func (rg *raftLog) maybeCommit(index, term uint64) bool {
	if index > rg.committedIndex && rg.zeroTermOnErrCompacted(rg.term(index)) == term {
		rg.commitTo(index)
		return true
	}
	return false
}

// appendToStorageUnstable appends new entries to unstable storage,
// and returns the last index of raftLog.
//
// (etcd raft.raftLog.append)
func (rg *raftLog) appendToStorageUnstable(entries ...raftpb.Entry) uint64 {
	if len(entries) == 0 {
		return rg.lastIndex()
	}

	if expectedLastIdx := entries[0].Index - 1; expectedLastIdx < rg.committedIndex {
		raftLogger.Panicf("expected last index '%d' is out of range on raft log committed index '%d'", expectedLastIdx, rg.committedIndex)
	}

	rg.storageUnstable.truncateAndAppend(entries...)
	return rg.lastIndex()
}

// findConflictingTerm finds the first entry index with conflicting term.
// An entry is conflicting if it has the same index but different term.
// If the given entries contain new entries, which still does not match in
// the terms with those extra entries, it returns the index of first new entry.
// The index of given entries must be continuously increasing.
//
// (etcd raft.raftLog.findConflict)
func (rg *raftLog) findConflictingTerm(entries ...raftpb.Entry) uint64 {
	for _, ent := range entries {
		if !rg.matchTerm(ent.Index, ent.Term) {
			if ent.Index <= rg.lastIndex() {
				raftLogger.Infof("conflicting entry at index %d [existing term %d != conflicting term %d]",
					ent.Index, rg.zeroTermOnErrCompacted(rg.term(ent.Index)), ent.Term)
			}
			return ent.Index
		}
	}
	return 0
}

// maybeAppend returns the last index of new entries and true, if appends were successful.
// Otherwise, it returns 0 and false.
//
// (etcd raft.raftLog.maybeAppend)
func (rg *raftLog) maybeAppend(index, term, committedIndex uint64, entries ...raftpb.Entry) (uint64, bool) {
	if rg.matchTerm(index, term) {
		conflictingIndex := rg.findConflictingTerm(entries...)
		switch {
		case conflictingIndex == 0:

		case conflictingIndex <= rg.committedIndex:
			raftLogger.Panicf("conflicting entry index %d must be greater than committed index %d", conflictingIndex, rg.committedIndex)

		default:
			// For example
			//
			// entries in unstable = [10(term 7), 11(term 7), 12(term 8)]
			// index               = 10
			// term                =  7
			// committedIndex      = 12
			//
			// entries to append         = [11(term 8), 12(term 8)]
			// ➝ conflicting entry index = 11
			//
			// ➝ new start index
			//    = conflicting entry index - (index + 1)
			//    = 11 - (10 + 1)
			//    = 0
			//
			// ➝ entries to append = [11(term 8), 12(term 8)]
			//
			newStartIndex := conflictingIndex - (index + 1)
			rg.appendToStorageUnstable(entries[newStartIndex:]...)
			//
			// truncates...
			// ➝ entries in unstable = [10(term 7), 11(term 8), 12(term 8)]
		}

		// committedIndex = 12
		// lastNewIndex   = index + len(entries)
		//                = 10 + 2
		//                = 12
		// commitTo( min( committedIndex , lastNewIndex ) )
		// = commitTo( min(12,12) ) = commitTo(12)
		//
		lastNewIndex := index + uint64(len(entries))
		rg.commitTo(minUint64(committedIndex, lastNewIndex))
		return lastNewIndex, true
	}

	return 0, false
}
