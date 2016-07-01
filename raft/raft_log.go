package raft

import (
	"fmt"
	"math"

	"github.com/gyuho/db/raft/raftpb"
)

type raftLog struct {
	// storageStable contains all stable entries since the last snapshot.
	storageStable StorageStable

	// storageUnstable contains all unstable entries and snapshot to store into stableStorage.
	storageUnstable storageUnstable

	// committedIndex is the higest log position that is known to be stored in
	// the stable storage "on a quorum of nodes".
	committedIndex uint64

	// appliedIndex is the highest log position that the application has been
	// instructed to apply to its state machine.
	// Must: appliedIndex <= committedIndex
	appliedIndex uint64
}

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
	return fmt.Sprintf("[commit index = %d | applied index = %d | unstable.indexOffset = %d | len(unstanble.entries) = %d",
		rg.committedIndex, rg.appliedIndex,
		rg.storageUnstable.indexOffset, len(rg.storageUnstable.entries),
	)
}

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

func (rg *raftLog) term(index uint64) (uint64, error) {
	dummyIndex := rg.firstIndex() - 1
	if index < dummyIndex || index > rg.lastIndex() {
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

func (rg *raftLog) lastTerm() uint64 {
	tm, err := rg.term(rg.lastIndex())
	if err != nil {
		raftLogger.Panicf("raftLog.term(rg.lastIndex) error (%v)", err)
	}
	return tm
}

// matchTerm returns true if log index does actually has the 'term'.
func (rg *raftLog) matchTerm(index, term uint64) bool {
	tm, err := rg.term(index)
	if err != nil {
		return false
	}
	return tm == term
}

// mustCheckSliceBoundary ensures that:
//
//   rg.firstIndex() <= startIndex <= endIndex <= rg.firstIndex() + len(rg.entries)
//
func (rg *raftLog) mustCheckSliceBoundary(startIndex, endIndex uint64) error {
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
func (rg *raftLog) slice(startIndex, endIndex, limitSize uint64) ([]raftpb.Entry, error) {
	if err := rg.mustCheckSliceBoundary(startIndex, endIndex); err != nil {
		return nil, err
	}

	if startIndex == endIndex {
		return nil, nil
	}

	var entries []raftpb.Entry

	if startIndex < rg.storageUnstable.indexOffset { // try stable entries
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

		var (
			stableEntriesN = uint64(len(stableEntries))
			expectedN      = minUint64(rg.storageUnstable.indexOffset, endIndex) - startIndex
		)
		if stableEntriesN < expectedN { // no need to check limits
			return stableEntries, nil
		}

		entries = stableEntries
	}

	if endIndex > rg.storageUnstable.indexOffset {
		unstableEntries := rg.storageUnstable.slice(maxUint64(startIndex, rg.storageUnstable.indexOffset), endIndex)
		if len(entries) > 0 { // there are entries from stable storage
			entries = append(entries, unstableEntries...)
		} else {
			entries = unstableEntries
		}
	}

	return limitEntries(entries, limitSize), nil
}

// entries returns the entries[startIndex,rg.endIndex+1) with size limit.
func (rg *raftLog) entries(startIndex, limitSize uint64) ([]raftpb.Entry, error) {
	if startIndex > rg.lastIndex() {
		return nil, nil
	}
	return rg.slice(startIndex, rg.lastIndex()+1, limitSize)
}

func (rg *raftLog) allEntries() []raftpb.Entry {
	entries, err := rg.entries(rg.firstIndex(), math.MaxUint64)
	if err != nil {
		switch err {
		case ErrCompacted: // try again in case there was a racing compaction
			return rg.allEntries()
		default:
			raftLogger.Panicf("allEntries error (%v)", err)
		}
	}
	return entries
}
