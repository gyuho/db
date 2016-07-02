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
		raftLogger.Warningf("index '%d' is out of range [dummy index=%d, last index=%d]", index, dummyIndex, rg.lastIndex())
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
		raftLogger.Panicf("invalid raft log indexes [start index = %d | end index = %d]", startIndex, endIndex)
	}

	firstIndex := rg.firstIndex()
	if firstIndex > startIndex {
		return ErrCompacted
	}

	entryN := rg.lastIndex() - firstIndex + 1
	if endIndex > firstIndex+entryN {
		raftLogger.Panicf("entries[%d, %d) is out of bound [first index = %d | last index = %d]", startIndex, endIndex, firstIndex, rg.lastIndex())
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
	// entries in stable storage        = [10, 11, 12]
	// entries in unstable storage      = [13, 14]
	// index offset in unstable storage = 13
	//
	// slice(startIndex=11, endIndex=15)
	//
	// ➝ startIndex < index offset in unstable storage
	// ➝         11 < 13
	//
	// ➝ stableStorage.Entries( startIndex , min( endIndex , index offset in unstable storage ) )
	// ➝ stableStorage.Entries( 11 , min (15,13) )
	// ➝ stableStorage.Entries( 11 , 13 )
	//   == stableStorage.Entries[startIndex, endIndex)
	// ➝ [11, 12]
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

		// stableEntriesN = len([10, 11, 12]) = 3
		// expectedN      = min(index offset in unstable storage, endIndex) - startIndex
		//                = min(13, 15) - 10 = 3
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
		// entries in stable storage        = [10, 11, 12]
		// entries in unstable storage      = [14, 15]
		// index offset in unstable storage = 14
		//
		// slice(startIndex=11, endIndex=15)
		//
		// ➝ startIndex < index offset in unstable storage
		// ➝         11 < 14
		//
		// ➝ stableStorage.Entries( startIndex , min( endIndex , index offset in unstable storage ) )
		// ➝ stableStorage.Entries( 11 , min (15,14) )
		// ➝ stableStorage.Entries( 11 , 14 )
		//
		// endIndex > ms.lastIndex()+1 == 12 + 1 == 13
		//       14 > 13
		// ➝ out of bound panic!

		// For example,
		//
		// entries in stable storage        = [11, 12]
		// entries in unstable storage      = [13, 14]
		// index offset in unstable storage = 13
		//
		// slice(startIndex=11, endIndex=15)
		//
		// ➝ startIndex < index offset in unstable storage
		// ➝         11 < 13
		//
		// ➝ stableStorage.Entries( startIndex , min( endIndex , index offset in unstable storage ) )
		// ➝ stableStorage.Entries( 11 , min (15,13) )
		// ➝ stableStorage.Entries( 11 , 13 )
		// ➝ stableStorage.Entries[startIndex, endIndex)
		// ➝ [11, 12]
		//
		// stableEntriesN = len([11, 12]) = 2
		// expectedN      = min(index offset in unstable storage, endIndex) - startIndex
		//                = min(13, 15) - 10 = 3
		//
		// stableEntriesN < expectedN
		// ➝ no need to check limits
		//
		// ???
		// Then [13, 14] in unstable storage won't return

		entries = stableEntries
	}

	// For example,
	//
	// entries in stable storage        = [10, 11, 12]
	// entries in unstable storage      = [13, 14]
	// index offset in unstable storage = 13
	//
	// slice(startIndex=11, endIndex=15)
	//
	// ➝ startIndex < index offset in unstable storage
	// ➝         11 < 13
	//
	// ➝ stableStorage.Entries( startIndex , min( endIndex , index offset in unstable storage ) )
	// ➝ stableStorage.Entries( 11 , min (15,13) )
	// ➝ stableStorage.Entries( 11 , 13 )
	//   == stableStorage.Entries[startIndex, endIndex)
	// ➝ [11, 12]
	//
	//
	// ➝ endIndex > index offset in unstable storage
	// ➝       15 > 13
	//
	// ➝ ents = unstableEntries.slice( max( startIndex , index offset in unstable storage ) , endIndex )
	//        = unstableEntries.slice( max(13,13), 15 )
	//        = unstableEntries.slice(13, 15)
	//        = [13, 14]
	//
	// ➝ return [11, 12] + [13, 14]
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

/*
findConflict
maybeAppend
maybeCommit

isUpToDate

commitTo
appliedTo
stableTo
stableSnapTo
*/
