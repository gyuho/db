package raft

import (
	"fmt"
	"math"

	"github.com/gyuho/db/raft/raftpb"
)

// storageRaftLog represents raft log and its storage.
//
// (etcd raft.raftLog)
type storageRaftLog struct {
	// storageStable contains all stable entries since the last snapshot.
	storageStable StorageStable

	// storageUnstable contains all unstable entries and snapshot to store into stableStorage.
	// No need to define in pointer because 'storageRaftLog' will be passed with pointer.
	storageUnstable storageUnstable

	// committedIndex is the higest log position that is known to be stored in
	// the stable storage "on a quorum of nodes".
	committedIndex uint64

	// appliedIndex is the highest log position that the application has been
	// instructed to apply to its state machine.
	// Must: appliedIndex <= committedIndex
	appliedIndex uint64
}

// newStorageRaftLog returns a new storageRaftLog with specified stable storage.
//
// (etcd raft.raftLog.newLog)
func newStorageRaftLog(storageStable StorageStable) *storageRaftLog {
	if storageStable == nil {
		raftLogger.Panic("stable storage must not be nil")
	}

	sr := &storageRaftLog{
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

	sr.storageUnstable.snapshot = nil
	sr.storageUnstable.indexOffset = lastIndex + 1
	sr.storageUnstable.entries = nil

	// index of last compaction
	sr.committedIndex = firstIndex - 1
	sr.appliedIndex = firstIndex - 1

	return sr
}

func (sr *storageRaftLog) String() string {
	return fmt.Sprintf("[committed index=%d | applied index=%d | unstable.indexOffset=%d | len(unstanble.entries)=%d]",
		sr.committedIndex, sr.appliedIndex,
		sr.storageUnstable.indexOffset, len(sr.storageUnstable.entries),
	)
}

// firstIndex gets the first index from unstable storage first.
// If snapshot exists, it returns ms.snapshotEntries[0].Index + 1.
// If it's not available, try to get the first index in stable storage.
//
// (etcd raft.raftLog.firstIndex)
func (sr *storageRaftLog) firstIndex() uint64 {
	if index, ok := sr.storageUnstable.maybeFirstIndex(); ok {
		return index
	}

	index, err := sr.storageStable.FirstIndex()
	if err != nil {
		raftLogger.Panicf("raftLog.storageStable.FirstIndex error (%v)", err)
	}

	return index
}

// lastIndex gets the last index from unstable storage first.
// If it's not available, try to get the last index in stable storage.
//
// (etcd raft.raftLog.lastIndex)
func (sr *storageRaftLog) lastIndex() uint64 {
	if index, ok := sr.storageUnstable.maybeLastIndex(); ok {
		return index
	}

	index, err := sr.storageStable.LastIndex()
	if err != nil {
		raftLogger.Panicf("raftLog.storageStable.LastIndex error (%v)", err)
	}

	return index
}

// term gets the term of specified index from unstable storage first.
// If it's not available, try to get the term in stable storage.
//
// (etcd raft.raftLog.term)
func (sr *storageRaftLog) term(index uint64) (uint64, error) {
	dummyIndex := sr.firstIndex() - 1
	if index < dummyIndex || sr.lastIndex() < index {
		raftLogger.Warningf("out-of-range index '%d' [dummy index=%d | last index=%d], returning term '0'", index, dummyIndex, sr.lastIndex())
		return 0, nil
	}

	if tm, ok := sr.storageUnstable.maybeTerm(index); ok {
		return tm, nil
	}

	tm, err := sr.storageStable.Term(index)
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
func (sr *storageRaftLog) lastTerm() uint64 {
	tm, err := sr.term(sr.lastIndex())
	if err != nil {
		raftLogger.Panicf("raftLog.term(sr.lastIndex) error (%v)", err)
	}
	return tm
}

// matchTerm returns true if log index does actually has the 'term'.
//
// (etcd raft.raftLog.matchTerm)
func (sr *storageRaftLog) matchTerm(index, term uint64) bool {
	tm, err := sr.term(index)
	if err != nil {
		return false
	}
	return tm == term
}

// mustCheckOutOfBounds ensures that:
//
//   sr.firstIndex() <= startIndex <= endIndex <= sr.firstIndex() + len(sr.entries)
//
// (etcd raft.raftLog.mustCheckOutOfBounds)
func (sr *storageRaftLog) mustCheckOutOfBounds(startIndex, endIndex uint64) error {
	if startIndex > endIndex {
		raftLogger.Panicf("invalid raft log indexes [start index=%d | end index=%d]", startIndex, endIndex)
	}

	firstIndex := sr.firstIndex()
	if firstIndex > startIndex {
		return ErrCompacted
	}

	entryN := sr.lastIndex() - firstIndex + 1
	if endIndex > firstIndex+entryN {
		raftLogger.Panicf("entries[%d, %d) is out of bound [first index=%d | last index=%d]", startIndex, endIndex, firstIndex, sr.lastIndex())
	}

	return nil
}

// slice returns the entries[startIndex, endIndex) with limit size.
//
// (etcd raft.raftLog.slice)
func (sr *storageRaftLog) slice(startIndex, endIndex, limitSize uint64) ([]raftpb.Entry, error) {
	if err := sr.mustCheckOutOfBounds(startIndex, endIndex); err != nil {
		return nil, err
	}

	if startIndex == endIndex {
		return nil, nil
	}

	// entries is slice, so it's reference(pointer)
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
	if startIndex < sr.storageUnstable.indexOffset { // try stable storage entries
		stableEntries, err := sr.storageStable.Entries(startIndex, minUint64(endIndex, sr.storageUnstable.indexOffset), limitSize)
		if err != nil {
			switch err {
			case ErrCompacted:
				return nil, err
			case ErrUnavailable:
				raftLogger.Panicf("entries[%d,%d) is unavailable from stable storage", startIndex, minUint64(endIndex, sr.storageUnstable.indexOffset))
			default:
				raftLogger.Panicf("entries[%d,%d) is unavailable from stable storage (%v)", startIndex, minUint64(endIndex, sr.storageUnstable.indexOffset), err)
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
			expectedN      = minUint64(sr.storageUnstable.indexOffset, endIndex) - startIndex
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

		// entries is slice, so it's reference(pointer)
		// we need to be careful to not manipulate this
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
	if endIndex > sr.storageUnstable.indexOffset { // try unstable storage entries
		unstableEntries := sr.storageUnstable.slice(maxUint64(startIndex, sr.storageUnstable.indexOffset), endIndex)
		if len(entries) > 0 { // there are entries from stable storage
			// (X)
			// entries = append(entries, unstableEntries...)
			// this triggers race condition in etcd integration test!

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

	return limitEntries(limitSize, entries...), nil
}

// entries returns the entries[startIndex,sr.endIndex+1) with size limit.
//
// (etcd raft.raftLog.entries)
func (sr *storageRaftLog) entries(startIndex, limitSize uint64) ([]raftpb.Entry, error) {
	if startIndex > sr.lastIndex() {
		return nil, nil
	}
	return sr.slice(startIndex, sr.lastIndex()+1, limitSize)
}

// unstableEntries returns all unstable entries in raft log.
//
// (etcd raft.raftLog.unstableEntries)
func (sr *storageRaftLog) unstableEntries() []raftpb.Entry {
	if len(sr.storageUnstable.entries) == 0 {
		return nil
	}
	return sr.storageUnstable.entries
}

// allEntries returns all entries in raft log, except the first dummy entry.
//
// (etcd raft.raftLog.allEntries)
func (sr *storageRaftLog) allEntries() []raftpb.Entry {
	entries, err := sr.entries(sr.firstIndex(), math.MaxUint64)
	if err != nil {
		switch err {
		case ErrCompacted: // try again in case there was a racing compaction
			return sr.allEntries()
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
func (sr *storageRaftLog) snapshot() (raftpb.Snapshot, error) {
	if sr.storageUnstable.snapshot != nil { // try the unstable storage first
		return *sr.storageUnstable.snapshot, nil
	}

	return sr.storageStable.Snapshot()
}

// restoreSnapshot sets unstable storage with incoming snapshot.
//
// (etcd raft.raftLog.restore)
func (sr *storageRaftLog) restoreSnapshot(snap raftpb.Snapshot) {
	raftLogger.Infof("log %v is restoring the given snapshot [snapshot index=%d | snapshot term=%d]", sr, snap.Metadata.Index, snap.Metadata.Term)
	sr.committedIndex = snap.Metadata.Index
	sr.storageUnstable.restoreSnapshot(snap)
}

// hasNextEntriesToApply returns true if there are entries
// availablefor execution.
//
// (etcd raft.raftLog.hasNextEnts)
func (sr *storageRaftLog) hasNextEntriesToApply() (uint64, bool) {
	maxStart := maxUint64(sr.appliedIndex+1, sr.firstIndex())
	return maxStart, sr.committedIndex >= maxStart
}

// nextEntriesToApply returns all the available entries ready for applying(execution).
//
// (etcd raft.raftLog.nextEnts)
func (sr *storageRaftLog) nextEntriesToApply() []raftpb.Entry {
	maxStart, ok := sr.hasNextEntriesToApply()
	if !ok {
		return nil
	}

	entries, err := sr.slice(maxStart, sr.committedIndex+1, math.MaxUint64)
	if err != nil {
		raftLogger.Panicf("nextEntriesToApply error (%v)", err)
	}
	return entries
}

// zeroTermOnErrCompacted returns 0 if specified error is ErrCompacted.
//
// (etcd raft.raftLog.zeroTermOnErrCompacted)
func (sr *storageRaftLog) zeroTermOnErrCompacted(term uint64, err error) uint64 {
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

// isUpToDate returns true if the given (index, term) log is more
// up-to-date than the last entry in the existing logs.
// It returns true, first if the term is greater than the last term.
// Second if the index is greater than the last index.
//
// (etcd raft.raftLog.isUpToDate)
func (sr *storageRaftLog) isUpToDate(index, term uint64) bool {
	return term > sr.lastTerm() || (term == sr.lastTerm() && index >= sr.lastIndex())
}

// persistedEntriesAt updates unstable entries and indexes after persisting
// those unstable entries to stable storage.
//
// (etcd raft.raftLog.stableTo)
func (sr *storageRaftLog) persistedEntriesAt(indexToPersist, termToPersist uint64) {
	sr.storageUnstable.persistedEntriesAt(indexToPersist, termToPersist)
}

// persistedSnapshotAt updates snapshot metadata after processing the incoming snapshot.
//
// (etcd raft.raftLog.stableSnapTo)
func (sr *storageRaftLog) persistedSnapshotAt(indexToPersist uint64) {
	sr.storageUnstable.persistedSnapshotAt(indexToPersist)
}

// commitTo updates raftLog's committedIndex.
//
// (etcd raft.raftLog.commitTo)
func (sr *storageRaftLog) commitTo(indexToCommit uint64) {
	// to never decrease commit index
	if sr.committedIndex < indexToCommit {
		if sr.lastIndex() < indexToCommit {
			raftLogger.Panicf("got wrong commit index '%d', smaller than last index '%d' (possible log corruption, truncation, lost)",
				indexToCommit, sr.lastIndex())
		}
		sr.committedIndex = indexToCommit
	}
}

// appliedTo updates raftLog's appliedIndex.
//
// (etcd raft.raftLog.appliedTo)
func (sr *storageRaftLog) appliedTo(indexToApply uint64) {
	if indexToApply == 0 {
		return
	}

	// MUST "sr.committedIndex >= indexToApply"
	if sr.committedIndex < indexToApply || indexToApply < sr.appliedIndex {
		raftLogger.Panicf("got wrong applied index '%d' [commit index=%d | previous applied index=%d]",
			indexToApply, sr.committedIndex, sr.appliedIndex)
	}

	sr.appliedIndex = indexToApply
}

// maybeCommit is only successful if 'indexToCommit' is greater than current 'committedIndex',
// and the current term of 'indexToCommit' matches the 'termToCommit' without ErrCompacted.
//
// (etcd raft.raftLog.maybeCommit)
func (sr *storageRaftLog) maybeCommit(indexToCommit, termToCommit uint64) bool {
	if indexToCommit > sr.committedIndex && sr.zeroTermOnErrCompacted(sr.term(indexToCommit)) == termToCommit {
		sr.commitTo(indexToCommit)
		return true
	}
	return false
}

// appendToStorageUnstable appends new entries to unstable storage,
// and returns the last index of raftLog.
//
// (etcd raft.raftLog.append)
func (sr *storageRaftLog) appendToStorageUnstable(entries ...raftpb.Entry) uint64 {
	if len(entries) == 0 {
		return sr.lastIndex()
	}

	if expectedLastIdx := entries[0].Index - 1; expectedLastIdx < sr.committedIndex {
		raftLogger.Panicf("expected last index '%d' is out of range on raft log committed index '%d'", expectedLastIdx, sr.committedIndex)
	}

	sr.storageUnstable.truncateAndAppend(entries...)
	return sr.lastIndex()
}

// findConflict finds the first entry index with conflicting term.
// An entry is conflicting if it has the same index but different term.
// If the given entries contain new entries, which still does not match in
// the terms with those extra entries, it returns the index of first new entry.
// The index of given entries must be continuously increasing.
//
// (etcd raft.raftLog.findConflict)
func (sr *storageRaftLog) findConflict(entries ...raftpb.Entry) uint64 {
	for _, ent := range entries {
		if !sr.matchTerm(ent.Index, ent.Term) {
			if ent.Index <= sr.lastIndex() {
				raftLogger.Infof("conflicting entry at index %d [existing term %d != conflicting term %d]",
					ent.Index, sr.zeroTermOnErrCompacted(sr.term(ent.Index)), ent.Term)
			}
			return ent.Index
		}
	}
	return 0
}

// maybeAppend returns the last index of new entries and true, if successful.
// Otherwise, it returns 0 and false.
//
// (etcd raft.raftLog.maybeAppend)
func (sr *storageRaftLog) maybeAppend(index, term, indexToCommit uint64, entries ...raftpb.Entry) (uint64, bool) {
	if sr.matchTerm(index, term) {
		conflictingIndex := sr.findConflict(entries...)
		switch {
		case conflictingIndex == 0:

		case conflictingIndex <= sr.committedIndex:
			raftLogger.Panicf("conflicting entry index '%d' must be greater than committed index '%d'", conflictingIndex, sr.committedIndex)

		default:
			// For example
			//
			// entries in unstable = [10(term 7), 11(term 7), 12(term 8)]
			// index               = 10
			// term                =  7
			// indexToCommit       = 12
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
			sr.appendToStorageUnstable(entries[newStartIndex:]...)
			//
			// truncates...
			// ➝ entries in unstable = [10(term 7), 11(term 8), 12(term 8)]
		}

		// indexToCommit = 12
		// lastNewIndex  = index + len(entries)
		//               = 10 + 2
		//               = 12
		// commitTo( min( indexToCommit , lastNewIndex ) )
		// = commitTo( min(12,12) ) = commitTo(12)
		//
		lastNewIndex := index + uint64(len(entries))
		sr.commitTo(minUint64(indexToCommit, lastNewIndex))
		return lastNewIndex, true
	}

	return 0, false
}
