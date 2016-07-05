package raft

import (
	"sync"

	"github.com/gyuho/db/raft/raftpb"
)

// StorageStableInMemory implements StorageStable interface backed by in-memory storage.
type StorageStableInMemory struct {
	mu sync.Mutex

	hardState raftpb.HardState

	// snapshot contains metadata and encoded bytes data.
	snapshot raftpb.Snapshot

	// snapshotEntries[idx]'s raft log index == idx + snapshot.Metadata.Index
	snapshotEntries []raftpb.Entry
}

// NewStorageStableInMemory creates an empty StorageStable in memory.
func NewStorageStableInMemory() *StorageStableInMemory {
	return &StorageStableInMemory{
		// populate with one dummy entry at term 0
		snapshotEntries: make([]raftpb.Entry, 1),
	}
}

// GetState returns the saved HardState and ConfigState.
//
// (etcd raft.MemoryStorage.InitialState)
func (ms *StorageStableInMemory) GetState() (raftpb.HardState, raftpb.ConfigState, error) {
	return ms.hardState, ms.snapshot.Metadata.ConfigState, nil
}

func (ms *StorageStableInMemory) firstIndex() uint64 {
	return ms.snapshotEntries[0].Index + 1 // because first index is dummy at term 0
}

// FirstIndex returns the first index.
//
// (etcd raft.MemoryStorage.FirstIndex)
func (ms *StorageStableInMemory) FirstIndex() (uint64, error) {
	ms.mu.Lock()
	idx := ms.firstIndex()
	ms.mu.Unlock()

	return idx, nil
}

func (ms *StorageStableInMemory) lastIndex() uint64 {
	return ms.snapshotEntries[len(ms.snapshotEntries)-1].Index
}

// LastIndex returns the last index.
//
// (etcd raft.MemoryStorage.LastIndex)
func (ms *StorageStableInMemory) LastIndex() (uint64, error) {
	ms.mu.Lock()
	idx := ms.lastIndex()
	ms.mu.Unlock()

	return idx, nil
}

// Term returns the term of the given index.
//
// (etcd raft.MemoryStorage.Term)
func (ms *StorageStableInMemory) Term(index uint64) (uint64, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	firstEntryIndexInStorage := ms.firstIndex() - 1
	if firstEntryIndexInStorage > index {
		return 0, ErrCompacted
	}

	return ms.snapshotEntries[index-firstEntryIndexInStorage].Term, nil
}

// Entries returns the slice of stable storage log entries of [startIndex, endIndex).
//
// (etcd raft.MemoryStorage.Entries)
func (ms *StorageStableInMemory) Entries(startIndex, endIndex, limitSize uint64) ([]raftpb.Entry, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	firstEntryIndexInStorage := ms.firstIndex() - 1
	if firstEntryIndexInStorage >= startIndex { // == means match with the dummy entry
		return nil, ErrCompacted
	}

	// since [startIndex, endIndex)
	if endIndex > ms.lastIndex()+1 {
		raftLogger.Panicf("end index on '%d' out of bound (entries last index = %d)", endIndex, ms.lastIndex())
	}

	// only contain the dummy entry
	if len(ms.snapshotEntries) == 1 {
		return nil, ErrUnavailable
	}

	entries := ms.snapshotEntries[startIndex-firstEntryIndexInStorage : endIndex-firstEntryIndexInStorage]
	return limitEntries(entries, limitSize), nil
}

// Snapshot returns the snapshot of stable storage.
//
// (etcd raft.MemoryStorage.Snapshot)
func (ms *StorageStableInMemory) Snapshot() (raftpb.Snapshot, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	return ms.snapshot, nil
}

/////////////////////////////////////////////////////////////////
//                     More methods ↓                          //
//                                  ↓                          //
//                                  ↓                          //
/////////////////////////////////////////////////////////////////

// Append appends entries to storage. Make sure not to manipulate
// the original entries so to be optimized for returning.
//
// (etcd raft.MemoryStorage.Append)
func (ms *StorageStableInMemory) Append(entries ...raftpb.Entry) error {
	if len(entries) == 0 {
		return nil
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	// For example
	//
	// entries in snapshot      = [0, 11, 12]
	// entries passed to append = [9, 10]
	//
	// first log index  = 11
	// last entry index = 10
	//
	// first log index > last entry index
	// ➝ No new entry!
	//
	firstLogIndex := ms.firstIndex()
	lastEntryIndex := entries[len(entries)-1].Index
	if firstLogIndex > lastEntryIndex { // no new entry
		return nil
	}

	// For example
	//
	// entries in snapshot = [0, 11, 12]
	// entries in append   = [9, 10, 11, 12, 13, 14]
	//
	// first log   index = 11
	// first entry index =  9
	// last  entry index = 14
	//
	// first log index =< last entry index
	// ➝ New entry!
	//
	// first log index > first entry index
	// ➝ Need to truncate compacted entries!
	// ➝ entries = entries[first log index - first entry index:]
	//           == entries[11 - 9:] == entries[2:] == [11, 12, 13, 14]
	//
	firstEntryIndex := entries[0].Index
	if firstLogIndex > firstEntryIndex { // truncate compacted entries
		entries = entries[firstLogIndex-firstEntryIndex:]
	}

	// Now
	//
	// entries in snapshot = [0,  11, 12]
	// entries to append   = [11, 12, 13, 14]
	//
	// first log   index = 11
	// first entry index = 11
	//
	// offset = first entry index - first log index + 1
	//        = 1
	//
	// size of snapshot entries = 3
	//
	firstEntryIndex = entries[0].Index

	var (
		offset         = firstEntryIndex - firstLogIndex + 1
		snapshotEntryN = uint64(len(ms.snapshotEntries))
	)
	switch {
	case snapshotEntryN > offset:
		// ↳ Now entries in snapshot = [0, 11, 12]
		//
		// make a copy to not manipulate the original entries
		// (X) ms.snapshotEntries = ms.snapshotEntries[:offset]
		tmps := make([]raftpb.Entry, offset)
		copy(tmps, ms.snapshotEntries[:offset])
		ms.snapshotEntries = tmps
		//
		// ➝ Now entries in snapshot = [0]

		// Then [0] ← append [11, 12, 13, 14]
		// ➝ Now entries in snapshot = [0, 11, 12, 13, 14]
		//
		ms.snapshotEntries = append(ms.snapshotEntries, entries...)

	case snapshotEntryN == offset:
		//
		// For example
		//
		// entries in snapshot = [ 0, 11, 12]
		// entries in append   = [13, 14, 15, 16, 17]
		//
		// first log   index = 11
		// first entry index = 13
		// last  entry index = 17
		//
		// first log index =< last entry index
		// ➝ New entry!
		//
		// first log index < first entry index
		// ➝ No need to truncate compacted entries!
		//
		// offset = first entry index - first log index + 1
		//        = 13 - 11 + 1 = 3
		//
		// size of snapshot entries = 3
		//
		// offset == size of snapshot entries
		// ➝ Just append all entries!
		//
		ms.snapshotEntries = append(ms.snapshotEntries, entries...)

	default:
		raftLogger.Panicf("missing log entry [last log index: %d | entries[0].Index: %d]", ms.lastIndex(), entries[0].Index)
	}

	return nil
}

// CreateSnapshot makes, update snapshot in storage, later to be retrieved by the Snapshot() method.
// This is used to recontruct the point-in-time state of storage.
//
// (etcd raft.MemoryStorage.CreateSnapshot)
func (ms *StorageStableInMemory) CreateSnapshot(idx uint64, configState *raftpb.ConfigState, data []byte) (raftpb.Snapshot, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if idx <= ms.snapshot.Metadata.Index {
		return raftpb.Snapshot{}, ErrSnapOutOfDate
	}

	firstEntryIndexInStorage := ms.firstIndex() - 1
	if firstEntryIndexInStorage > ms.lastIndex() {
		raftLogger.Panicf("snapshot is out of bound [first log entry index in storage = %d | last log index in storage = %d]", firstEntryIndexInStorage, ms.lastIndex())
	}

	ms.snapshot.Metadata.Index = idx
	ms.snapshot.Metadata.Term = ms.snapshotEntries[idx-firstEntryIndexInStorage].Term

	if configState != nil {
		ms.snapshot.Metadata.ConfigState = *configState
	}

	ms.snapshot.Data = data

	return ms.snapshot, nil
}

// ApplySnapshot updates the snapshot in stable storage.
//
// (etcd raft.MemoryStorage.ApplySnapshot)
func (ms *StorageStableInMemory) ApplySnapshot(snapshot raftpb.Snapshot) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.snapshot = snapshot
	ms.snapshotEntries = []raftpb.Entry{ // metadata in the first entry as a dummy entry
		{Index: snapshot.Metadata.Index, Term: snapshot.Metadata.Term},
	}

	return nil
}

// SetHardState saves the current HardState.
//
// (etcd raft.MemoryStorage.SetHardState)
func (ms *StorageStableInMemory) SetHardState(state raftpb.HardState) error {
	ms.hardState = state
	return nil
}

// Compact discards all log entries "up to" compactIndex.
// It keeps entries[compactIndex:], and retains entries[compactIndex]
// in its first entry only for matching purposes.
//
// The application must ensure that it does not compacts on an index
// greater than applied index.
//
// (etcd raft.MemoryStorage.Compact)
func (ms *StorageStableInMemory) Compact(compactIndex uint64) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	firstEntryIndexInStorage := ms.firstIndex() - 1
	if firstEntryIndexInStorage >= compactIndex { // == means first dummy entry (already compacted)
		return ErrCompacted
	}

	if compactIndex > ms.lastIndex() {
		raftLogger.Panicf("compact on '%d' is out of bound (last log index in storage = %d)", compactIndex, ms.lastIndex())
	}

	// For example
	//
	// entries in snapshot           = [10, 11, 12, 13]
	// first entry index in snapshot = 10
	// compact index                 = 11
	// size of entries in snapshot   =  4
	//
	// new entry start index = compact index - first entry index in snapshot
	//                       = 11 - 10 = 1
	//
	// entries = entries[1:] == [11, 12, 13]
	//
	newEntryStartIndex := compactIndex - firstEntryIndexInStorage

	// DO NOT MODIFY THE ORIGINAL ENTRIES
	// (X) ms.snapshotEntries = ms.snapshotEntries[newEntryStartIndex:]

	// new capacity = size of entries in snapshot - new entry start index
	//              = 4 - 1 = 3
	//              = uint64(len(ms.snapshotEntries) - i)
	//
	// (etcd adds 1 more to the capacity)
	//
	tmps := make([]raftpb.Entry, 1, uint64(len(ms.snapshotEntries))-newEntryStartIndex)
	tmps[0].Index = ms.snapshotEntries[newEntryStartIndex].Index
	tmps[0].Term = ms.snapshotEntries[newEntryStartIndex].Term
	// skip tmps[0].Data

	tmps = append(tmps, ms.snapshotEntries[newEntryStartIndex+1:]...)

	ms.snapshotEntries = tmps

	return nil
}

func limitEntries(entries []raftpb.Entry, limitSize uint64) []raftpb.Entry {
	if len(entries) == 0 {
		return entries
	}

	var (
		total int
		i     int
	)
	for i = 0; i < len(entries); i++ {
		total += entries[i].Size()

		// to return at least one entry
		if i != 0 && uint64(total) > limitSize {
			break
		}
	}

	return entries[:i]
}
