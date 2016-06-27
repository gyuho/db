package raft

import (
	"sync"

	"github.com/gyuho/db/raft/raftpb"
)

// StorageInMemory implements Storage interface backed by in-memory storage.
type StorageInMemory struct {
	mu sync.Mutex

	hardState raftpb.HardState

	// snapshot contains metadata and encoded bytes data.
	snapshot raftpb.Snapshot

	// snapshotEntries[idx]'s raft log index == idx + snapshot.Metadata.Index
	snapshotEntries []raftpb.Entry
}

// NewStorageInMemory creates an empty Storage in memory.
func NewStorageInMemory() *StorageInMemory {
	return &StorageInMemory{
		// populate with one dummy entry at term 0
		snapshotEntries: make([]raftpb.Entry, 1),
	}
}

func (ms *StorageInMemory) GetState() (raftpb.HardState, *raftpb.ConfigState, error) {
	return ms.hardState, ms.snapshot.Metadata.ConfigState, nil
}

func (ms *StorageInMemory) Term(idx uint64) (uint64, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	firstEntryIndexInStorage := ms.firstIndex() - 1
	if firstEntryIndexInStorage > idx {
		return 0, ErrCompacted
	}

	return ms.snapshotEntries[idx-firstEntryIndexInStorage].Term, nil
}

func (ms *StorageInMemory) firstIndex() uint64 {
	return ms.snapshotEntries[0].Index + 1 // because first index is dummy at term 0
}

func (ms *StorageInMemory) FirstIndex() (uint64, error) {
	ms.mu.Lock()
	idx := ms.firstIndex()
	ms.mu.Unlock()

	return idx, nil
}

func (ms *StorageInMemory) lastIndex() uint64 {
	return ms.snapshotEntries[len(ms.snapshotEntries)-1].Index
}

func (ms *StorageInMemory) LastIndex() (uint64, error) {
	ms.mu.Lock()
	idx := ms.lastIndex()
	ms.mu.Unlock()

	return idx, nil
}

func (ms *StorageInMemory) Entries(startIdx, endIdx, limitSize uint64) ([]raftpb.Entry, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	firstEntryIndexInStorage := ms.firstIndex() - 1
	if firstEntryIndexInStorage >= startIdx { // == means match with the dummy entry
		return nil, ErrCompacted
	}

	// since [startIdx, endIdx)
	if endIdx > ms.lastIndex()+1 {
		raftLogger.Panicf("end index on '%d' out of bound (entries last index = %d)", endIdx, ms.lastIndex())
	}

	// only contain the dummy entry
	if len(ms.snapshotEntries) == 1 {
		return nil, ErrUnavailable
	}

	entries := ms.snapshotEntries[startIdx-firstEntryIndexInStorage : endIdx-firstEntryIndexInStorage]
	return limitEntries(entries, limitSize), nil
}

func (ms *StorageInMemory) Snapshot() (raftpb.Snapshot, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	return ms.snapshot, nil
}

func (ms *StorageInMemory) Append(entries []raftpb.Entry) error {
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
	// ==> No new entry!
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
	// ==> New entry!
	//
	// first log index > first entry index
	// ==> Need to truncate compacted entries!
	// ==> entries = entries[first log index - first entry index:]
	//             == entries[11 - 9:] == entries[2:] == [11, 12, 13, 14]
	//
	firstEntryIndex := entries[0].Index
	if firstLogIndex > firstEntryIndex { // truncate compacted entries
		entries = entries[firstLogIndex-firstEntryIndex:]
	}

	// Now
	//
	// entries in snapshot = [0,  11, 12]
	// entries in append   = [11, 12, 13, 14]
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
		//
		// Now entries in snapshot = [0, 11, 12]
		ms.snapshotEntries = ms.snapshotEntries[:offset]
		// Now entries in snapshot = [0]
		//
		// Then append [11, 12, 13, 14] to [0]
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
		// ==> New entry!
		//
		// first log index < first entry index
		// ==> No need to truncate compacted entries!
		//
		// offset = first entry index - first log index + 1
		//        = 13 - 11 + 1 = 3
		//
		// size of snapshot entries = 3
		//
		// offset == size of snapshot entries
		// ==> Just append all entries!
		//
		ms.snapshotEntries = append(ms.snapshotEntries, entries...)
	}

	return nil
}

// CreateSnapshot makes, update snapshot in storage, later to be retrieved by the Snapshot() method.
// This is used to recontruct the point-in-time state of storage.
func (ms *StorageInMemory) CreateSnapshot(idx uint64, configState *raftpb.ConfigState, data []byte) (raftpb.Snapshot, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if idx <= ms.snapshot.Metadata.Index {
		return raftpb.Snapshot{}, ErrSnapOutOfDate
	}

	firstEntryIndexInStorage := ms.firstIndex() - 1
	if firstEntryIndexInStorage > ms.lastIndex() {
		raftLogger.Panicf("snapshot on '%d' is out of bound (last log index in storage = %d)", idx, ms.lastIndex())
	}

	ms.snapshot.Metadata.Term = ms.snapshotEntries[idx-firstEntryIndexInStorage].Term
	ms.snapshot.Metadata.Index = idx

	if configState != nil {
		ms.snapshot.Metadata.ConfigState = configState
	}

	ms.snapshot.Data = data

	return ms.snapshot, nil
}

func (ms *StorageInMemory) SetSnapshot(snapshot raftpb.Snapshot) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.snapshot = snapshot
	ms.snapshotEntries = []raftpb.Entry{
		{Term: snapshot.Metadata.Term, Index: snapshot.Metadata.Index},
	}

	return nil
}

func (ms *StorageInMemory) SetHardState(state raftpb.HardState) error {
	ms.hardState = state
	return nil
}

// Compact discards all log entries prior to compactIndex.
// The application must ensure that it does not compacts on an index
// greater than applied index.
func (ms *StorageInMemory) Compact(compactIndex uint64) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	firstEntryIndexInStorage := ms.firstIndex() - 1
	if firstEntryIndexInStorage >= compactIndex {
		return ErrCompacted
	}

	if compactIndex > ms.lastIndex() {
		raftLogger.Panicf("compact on '%d' is out of bound (last log index in storage %d)", compactIndex, ms.lastIndex())
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
	ms.snapshotEntries = ms.snapshotEntries[newEntryStartIndex:]
	//
	// ???
	// new capacity = size of entries in snapshot - new entry start index + 1
	//              = 4 - 1 = 3
	// newCapacity  = uint64(len(ms.snapshotEntries) - i)

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

		// at least one entry
		if i != 0 && uint64(total) > limitSize {
			break
		}
	}

	return entries[:i]
}
