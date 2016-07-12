package raft

import (
	"math"
	"reflect"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

func Test_storageRaftLog(t *testing.T) { // (etcd raft TestLogRestore)
	var (
		index    uint64 = 1000
		term     uint64 = 1000
		snapshot        = raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: index, Term: term}}
	)
	ms := NewStorageStableInMemory()
	ms.ApplySnapshot(snapshot)
	rg := newStorageRaftLog(ms)

	if entN := len(rg.allEntries()); entN != 0 {
		t.Fatalf("entry number expected 0, got %d", entN)
	}

	if fidx := rg.firstIndex(); fidx != index+1 { // get snapshot index + 1
		t.Fatalf("first index expected %d, got %d", index+1, fidx)
	}

	if rg.committedIndex != index { // newStorageRaftLog sets this
		t.Fatalf("committed index expected %d, got %d", index, rg.committedIndex)
	}

	if rg.storageUnstable.indexOffset != index+1 { // rg.storageUnstable.indexOffset = lastIndex + 1
		t.Fatalf("unstable index offset expected %d, got %d", index+1, rg.storageUnstable.indexOffset)
	}

	tm, err := rg.term(index)
	if err != nil {
		t.Fatal(err)
	}
	if tm != term {
		t.Fatalf("term for index %d expected %d, got %d", index, term, tm)
	}
}

func Test_storageRaftLog_mustCheckOutOfBounds(t *testing.T) { // (etcd raft TestIsOutOfBounds)
	var (
		indexOffset uint64 = 100
		snapshot           = raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: indexOffset}}
	)
	ms := NewStorageStableInMemory()
	ms.ApplySnapshot(snapshot)
	rg := newStorageRaftLog(ms)

	var (
		firstIndex        = indexOffset + 1
		num        uint64 = 100
	)
	for i := uint64(1); i <= num; i++ { // lasat index 200
		rg.appendToStorageUnstable(raftpb.Entry{Index: i + indexOffset})
	}

	tests := []struct {
		startIndex, endIndex uint64

		toPanic      bool
		errCompacted bool
	}{
		{
			firstIndex, firstIndex,

			false,
			false,
		},

		{
			firstIndex + num/2, firstIndex + num/2,

			false,
			false,
		},

		{
			firstIndex + num - 1, firstIndex + num - 1,

			false,
			false,
		},

		{
			firstIndex + num, firstIndex + num, // endIndex <= rg.firstIndex() + len(rg.entries) == 201

			false,
			false,
		},

		{
			firstIndex - 1, firstIndex + 1,

			false,
			true, // if firstIndex > startIndex { return ErrCompacted }
		},

		{
			firstIndex - 2, firstIndex + 1,

			false,
			true, // if firstIndex > startIndex { return ErrCompacted }
		},

		{
			firstIndex + num, firstIndex + num + 1, // out of bound with index 202

			true,
			false,
		},

		{
			firstIndex + num + 1, firstIndex + num + 1, // out of bound with index 202

			true,
			false,
		},
	}

	for i, tt := range tests {
		func() {
			defer func() {
				err := recover()
				if err != nil {
					t.Logf("#%d: panic with %v", i, err)
				}

				switch {
				case err == nil && tt.toPanic:
					t.Fatalf("#%d: expected panic but didn't", i)

				case err != nil && !tt.toPanic:
					t.Fatalf("#%d: expected no panic but got panic error (%v)", i, err)
				}
			}()

			if err := rg.mustCheckOutOfBounds(tt.startIndex, tt.endIndex); tt.errCompacted && err != ErrCompacted {
				t.Fatalf("#%d: error expected %v, got %v", i, ErrCompacted, err)
			}
		}()
	}
}

func Test_storageRaftLog_slice(t *testing.T) { // (etcd raft TestSlice)
	var (
		indexOffset uint64 = 100
		snapshot           = raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: indexOffset}}
		num         uint64 = 100
		midIndex           = indexOffset + num/2 // 150
		lastIndex          = indexOffset + num   // 200

		midIndexEntry = raftpb.Entry{Index: midIndex, Term: midIndex}
	)
	ms := NewStorageStableInMemory()
	ms.ApplySnapshot(snapshot)
	for i := uint64(1); i < num/2; i++ {
		ms.Append(raftpb.Entry{Index: indexOffset + i, Term: indexOffset + i})
	}
	rg := newStorageRaftLog(ms)

	for i := num / 2; i <= num; i++ {
		rg.appendToStorageUnstable(raftpb.Entry{Index: indexOffset + i, Term: indexOffset + i})
	}
	// rg.storageUnstable.indexOffset == 150

	tests := []struct {
		startIndex, endIndex uint64
		limitSize            uint64

		wEntries []raftpb.Entry
		wError   error

		toPanic bool
	}{
		{ // no limit, ErrCompacted
			indexOffset - 1, indexOffset + 1,
			math.MaxUint64,

			nil,
			ErrCompacted, // if firstIndex > startIndex { return ErrCompacted }
			// firstIndex 100 + 1 > startIndex 99

			false,
		},

		{ // no limit, ErrCompacted
			indexOffset, indexOffset + 1,
			math.MaxUint64,

			nil,
			ErrCompacted, // if firstIndex > startIndex { return ErrCompacted }
			// firstIndex 100 + 1 > startIndex 100

			false,
		},

		{ // no limit
			midIndex - 1, midIndex + 1,
			math.MaxUint64,

			// slice returns the entries[startIndex, endIndex)
			[]raftpb.Entry{{Index: midIndex - 1, Term: midIndex - 1}, {Index: midIndex, Term: midIndex}},
			nil,

			false,
		},

		{ // no limit
			midIndex - 3, midIndex + 2, // [147, 148, 149] + [150, 151]
			math.MaxUint64,

			// slice returns the entries[startIndex, endIndex)
			[]raftpb.Entry{
				{Index: midIndex - 3, Term: midIndex - 3},
				{Index: midIndex - 2, Term: midIndex - 2},
				{Index: midIndex - 1, Term: midIndex - 1},
				{Index: midIndex, Term: midIndex},
				{Index: midIndex + 1, Term: midIndex + 1},
			},
			nil,

			false,
		},

		{ // no limit
			midIndex, midIndex + 1,
			math.MaxUint64,

			// slice returns the entries[startIndex, endIndex)
			[]raftpb.Entry{{Index: midIndex, Term: midIndex}},
			nil,

			false,
		},

		{ // no limit
			lastIndex - 1, lastIndex + 1,
			math.MaxUint64,

			// slice returns the entries[startIndex, endIndex)
			[]raftpb.Entry{{Index: lastIndex - 1, Term: lastIndex - 1}, {Index: lastIndex, Term: lastIndex}},
			nil,

			false,
		},

		{ // no limit
			lastIndex - 1, lastIndex,
			math.MaxUint64,

			// slice returns the entries[startIndex, endIndex)
			[]raftpb.Entry{{Index: lastIndex - 1, Term: lastIndex - 1}},
			nil,

			false,
		},

		{ // no limit
			lastIndex, lastIndex + 1,
			math.MaxUint64,

			// slice returns the entries[startIndex, endIndex)
			[]raftpb.Entry{{Index: lastIndex, Term: lastIndex}},
			nil,

			false,
		},

		{ // no limit
			lastIndex + 1, lastIndex + 1,
			math.MaxUint64,

			// slice returns the entries[startIndex, endIndex)
			nil,
			nil,

			false,
		},

		{ // no limit, and expect panic
			lastIndex - 5, lastIndex + 2, // MUST "endIndex <= rg.firstIndex() + len(rg.entries)"
			math.MaxUint64,

			// slice returns the entries[startIndex, endIndex)
			nil,
			nil,

			true,
		},

		{ // no limit, and expect panic
			lastIndex + 1, lastIndex + 2, // MUST "endIndex <= rg.firstIndex() + len(rg.entries)"
			math.MaxUint64,

			// slice returns the entries[startIndex, endIndex)
			nil,
			nil,

			true,
		},

		{ // with limit 0, but to ensure that it returns at least one entry
			midIndex - 1, midIndex + 1,
			0,

			// slice returns the entries[startIndex, endIndex)
			// []raftpb.Entry{{Index: midIndex - 1, Term: midIndex - 1}, {Index: midIndex, Term: midIndex}},
			[]raftpb.Entry{{Index: midIndex - 1, Term: midIndex - 1}},
			nil,

			false,
		},

		{ // with limit size of only one entry
			midIndex - 1, midIndex + 1,
			uint64(midIndexEntry.Size() + 1),

			// slice returns the entries[startIndex, endIndex)
			// []raftpb.Entry{{Index: midIndex - 1, Term: midIndex - 1}, {Index: midIndex, Term: midIndex}},
			[]raftpb.Entry{{Index: midIndex - 1, Term: midIndex - 1}},
			nil,

			false,
		},

		{ // with limit size of 1 entry
			midIndex - 2, midIndex + 1,
			uint64(midIndexEntry.Size() + 1),

			// slice returns the entries[startIndex, endIndex)
			[]raftpb.Entry{{Index: midIndex - 2, Term: midIndex - 2}},
			nil,

			false,
		},

		{ // with limit size of 2 entries
			midIndex - 1, midIndex + 1,
			uint64(midIndexEntry.Size() * 2),

			// slice returns the entries[startIndex, endIndex)
			[]raftpb.Entry{{Index: midIndex - 1, Term: midIndex - 1}, {Index: midIndex, Term: midIndex}},
			nil,

			false,
		},

		{ // with limit size of 3 entries
			midIndex - 1, midIndex + 2,
			uint64(midIndexEntry.Size() * 3),

			// slice returns the entries[startIndex, endIndex)
			[]raftpb.Entry{{Index: midIndex - 1, Term: midIndex - 1}, {Index: midIndex, Term: midIndex}, {Index: midIndex + 1, Term: midIndex + 1}},
			nil,

			false,
		},

		{ // with limit size of 1 entry
			midIndex, midIndex + 2,
			uint64(midIndexEntry.Size()),

			// slice returns the entries[startIndex, endIndex)
			[]raftpb.Entry{{Index: midIndex, Term: midIndex}},
			nil,

			false,
		},

		{ // with limit size of 2 entries
			midIndex, midIndex + 2,
			uint64(midIndexEntry.Size() * 2),

			// slice returns the entries[startIndex, endIndex)
			[]raftpb.Entry{{Index: midIndex, Term: midIndex}, {Index: midIndex + 1, Term: midIndex + 1}},
			nil,

			false,
		},
	}

	for i, tt := range tests {
		func() {
			defer func() {
				err := recover()
				if err != nil {
					t.Logf("#%d: panic with %v", i, err)
				}

				switch {
				case err == nil && tt.toPanic:
					t.Fatalf("#%d: expected panic but didn't", i)

				case err != nil && !tt.toPanic:
					t.Fatalf("#%d: expected no panic but got panic error (%v)", i, err)
				}
			}()

			entries, err := rg.slice(tt.startIndex, tt.endIndex, tt.limitSize)
			if err != tt.wError {
				t.Fatalf("#%d: error expected %v, got %v", i, tt.wError, err)
			}

			if !reflect.DeepEqual(entries, tt.wEntries) {
				t.Fatalf("#%d: entries expected %+v, got %+v", i, tt.wEntries, entries)
			}
		}()
	}
}

func Test_storageRaftLog_unstableEntries(t *testing.T) { // (etcd raft TestUnstableEnts)
	tests := []struct {
		existingEntriesStorageStable   []raftpb.Entry
		entriesStorageUnstableToAppend []raftpb.Entry

		storageUnstableIndexOffset uint64
		wEntriesStorageUnstable    []raftpb.Entry
	}{
		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},
			nil,

			3,
			nil,
		},

		{
			nil,
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},

			1,
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},
		},
	}
	for i, tt := range tests {
		ms := NewStorageStableInMemory()
		ms.Append(tt.existingEntriesStorageStable...)

		rg := newStorageRaftLog(ms)
		rg.appendToStorageUnstable(tt.entriesStorageUnstableToAppend...)

		uents := rg.unstableEntries()
		if len(uents) > 0 {
			rg.persistedEntriesAt(uents[len(uents)-1].Index, uents[len(uents)-1].Term)
		}

		if !reflect.DeepEqual(uents, tt.wEntriesStorageUnstable) {
			t.Fatalf("#%d: unstable entries expected %+v, got %+v", i, tt.wEntriesStorageUnstable, uents)
		}

		var nextIdx uint64
		if len(tt.existingEntriesStorageStable) > 0 {
			nextIdx = tt.existingEntriesStorageStable[len(tt.existingEntriesStorageStable)-1].Index + 1
		} else if len(tt.entriesStorageUnstableToAppend) > 0 {
			nextIdx = tt.entriesStorageUnstableToAppend[0].Index + uint64(len(tt.entriesStorageUnstableToAppend))
		}
		if rg.storageUnstable.indexOffset != nextIdx {
			t.Fatalf("#%d: unstable index offset expected %d, got %d", i, nextIdx, rg.storageUnstable.indexOffset)
		}
	}
}

func Test_storageRaftLog_NextEntries(t *testing.T) { // (etcd raft TestHasNextEnts, TestNextEnts)
	tests := []struct {
		snapshotToApply                raftpb.Snapshot
		entriesStorageUnstableToAppend []raftpb.Entry

		indexToCommit, termToCommit uint64

		indexToApply uint64

		nextEntryIndexToApply uint64
		hasNextEntriesToApply bool
		nextEntriesToApply    []raftpb.Entry
	}{
		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 3, Term: 1}}, // firstIndex-1 is 3 from snapshot in this case
			[]raftpb.Entry{{Index: 4, Term: 1}, {Index: 5, Term: 1}, {Index: 6, Term: 1}},

			5, 1,

			0,

			4, true, // maxStart := maxUint64(rg.appliedIndex+1, rg.firstIndex())
			[]raftpb.Entry{{Index: 4, Term: 1}, {Index: 5, Term: 1}}, // because Index: 6 is not comitted yet
		},

		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 3, Term: 1}}, // firstIndex-1 is 3 from snapshot in this case
			[]raftpb.Entry{{Index: 4, Term: 1}, {Index: 5, Term: 1}, {Index: 6, Term: 1}},

			5, 1,

			3, // 2, then panic because applied index is already 3

			4, true, // maxStart := maxUint64(rg.appliedIndex+1, rg.firstIndex())
			[]raftpb.Entry{{Index: 4, Term: 1}, {Index: 5, Term: 1}}, // because Index: 6 is not comitted yet
		},

		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 3, Term: 1}}, // firstIndex-1 is 3 from snapshot in this case
			[]raftpb.Entry{{Index: 4, Term: 1}, {Index: 5, Term: 1}, {Index: 6, Term: 1}},

			5, 1,

			4,

			5, true, // maxStart := maxUint64(rg.appliedIndex+1, rg.firstIndex())
			[]raftpb.Entry{{Index: 5, Term: 1}}, // because Index: 6 is not comitted yet
		},

		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 3, Term: 1}}, // firstIndex-1 is 3 from snapshot in this case
			[]raftpb.Entry{{Index: 4, Term: 1}, {Index: 5, Term: 1}, {Index: 6, Term: 1}},

			5, 1,

			5,

			6, false, // maxStart := maxUint64(rg.appliedIndex+1, rg.firstIndex())
			nil, // because Index: 6 is not comitted yet
		},
	}
	for i, tt := range tests {
		st := NewStorageStableInMemory()
		st.ApplySnapshot(tt.snapshotToApply)

		// rg.committedIndex = firstIndex - 1 == 3
		// rg.appliedIndex   = firstIndex - 1 == 3
		rg := newStorageRaftLog(st)

		rg.appendToStorageUnstable(tt.entriesStorageUnstableToAppend...)

		// MUST "index > rg.committedIndex && rg.zeroTermOnErrCompacted(rg.term(index)) == term"
		rg.maybeCommit(tt.indexToCommit, tt.termToCommit) // sets committedIndex to 5

		rg.appliedTo(tt.indexToApply) // apply index should be smaller than committedIndex

		// hasNextEntriesToApply returns maxUint64(rg.appliedIndex+1, rg.firstIndex())
		nidx, hasNext := rg.hasNextEntriesToApply()
		if nidx != tt.nextEntryIndexToApply {
			t.Fatalf("#%d: next entry index to apply expected %v, got %v", i, tt.nextEntryIndexToApply, nidx)
		}
		if hasNext != tt.hasNextEntriesToApply {
			t.Fatalf("#%d: hasNextEntries expected %v, got %v", i, tt.hasNextEntriesToApply, hasNext)
		}

		nents := rg.nextEntriesToApply()
		if !reflect.DeepEqual(nents, tt.nextEntriesToApply) {
			t.Fatalf("#%d: next entries to apply expected %+v, got %+v", i, tt.nextEntriesToApply, nents)
		}
	}
}

func Test_storageRaftLog_isUpToDate(t *testing.T) { // (etcd raft TestIsUpToDate)
	tests := []struct {
		entriesStorageUnstableToAppend []raftpb.Entry
		index, term                    uint64

		upToDate bool
	}{
		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			2, 4, // term is greater

			true,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			3, 4, // term is greater

			true,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			4, 4, // term is greater

			true,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			2, 2, // smaller term, so index is ignored

			false,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			3, 2, // smaller term, so index is ignored

			false,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			4, 2, // smaller term, so index is ignored

			false,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			2, 3, // equal term, so equal or larger index makes true

			false,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			3, 3, // equal term, so equal or larger index makes true

			true,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			4, 3, // equal term, so equal or larger index makes true

			true,
		},
	}

	for i, tt := range tests {
		rg := newStorageRaftLog(NewStorageStableInMemory())
		rg.appendToStorageUnstable(tt.entriesStorageUnstableToAppend...)

		// isUpToDate returns true if the given (index, term) log is more
		// up-to-date than the last entry in the existing logs.
		// It returns true, first if the term is greater than the last term.
		// Second if the index is greater than the last index.
		upToDate := rg.isUpToDate(tt.index, tt.term)
		if upToDate != tt.upToDate {
			t.Fatalf("#%d: up-to-date expected %v, got %v", i, tt.upToDate, upToDate)
		}
	}
}

func Test_storageRaftLog_appendToStorageUnstable(t *testing.T) { // (etcd raft TestAppend)
	tests := []struct {
		existingEntriesStorageStable   []raftpb.Entry
		entriesStorageUnstableToAppend []raftpb.Entry

		wNewIndexAfterAppend    uint64
		wNewLogEntries          []raftpb.Entry
		wNewUnstableIndexOffset uint64
	}{
		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},
			[]raftpb.Entry{},

			2,
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},
			3, // because newStorageRaftLog makes "rg.storageUnstable.indexOffset = lastIndex + 1"
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},
			[]raftpb.Entry{{Index: 3, Term: 2}},

			3,
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 2}},
			3, // because newStorageRaftLog makes "rg.storageUnstable.indexOffset = lastIndex + 1"
		},

		{ // conflicting entry at index 1
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},
			[]raftpb.Entry{{Index: 1, Term: 2}},

			1,
			[]raftpb.Entry{{Index: 1, Term: 2}},
			1, // because truncateAndAppend makes "su.indexOffset = firstIndexInEntriesToAppend"
		},

		{ // conflicting entry at index 2
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},
			[]raftpb.Entry{{Index: 2, Term: 3}, {Index: 3, Term: 3}},

			3,
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 3}, {Index: 3, Term: 3}},
			2, // because truncateAndAppend makes "su.indexOffset = firstIndexInEntriesToAppend"
		},
	}

	for i, tt := range tests {
		ms := NewStorageStableInMemory()
		ms.Append(tt.existingEntriesStorageStable...)
		rg := newStorageRaftLog(ms)

		nindex := rg.appendToStorageUnstable(tt.entriesStorageUnstableToAppend...)
		if nindex != tt.wNewIndexAfterAppend {
			t.Fatalf("#%d: new index expected %d, got %d", i, tt.wNewIndexAfterAppend, nindex)
		}

		ents, err := rg.entries(1, math.MaxUint64)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(ents, tt.wNewLogEntries) {
			t.Fatalf("#%d: log entries expected %+v, got %+v", i, tt.wNewLogEntries, ents)
		}

		if noff := rg.storageUnstable.indexOffset; noff != tt.wNewUnstableIndexOffset {
			t.Fatalf("#%d: unstable storage offset expected %d, got %d", i, tt.wNewUnstableIndexOffset, noff)
		}
	}
}

func Test_storageRaftLog_findConflict(t *testing.T) { // (etcd raft TestFindConflict)
	tests := []struct {
		entriesStorageUnstableToAppend []raftpb.Entry
		entriesToCompare               []raftpb.Entry

		firstConflictingEntryIndex uint64
	}{
		{ // no conflict, because it's empty
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			[]raftpb.Entry{},
			0,
		},

		{ // no conflict, because they are equal
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			0,
		},

		{ // no conflict, because they have equal terms
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			[]raftpb.Entry{{Index: 2, Term: 2}, {Index: 3, Term: 3}},
			0,
		},

		{ // no conflict, because they have equal terms
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			[]raftpb.Entry{{Index: 3, Term: 3}},
			0,
		},

		{ // no conflict, but with new entries
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
			4, // terms with those extra entries, it returns the index of first new entry
		},

		{ // no conflict, but with new entries
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			[]raftpb.Entry{{Index: 2, Term: 2}, {Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
			4, // terms with those extra entries, it returns the index of first new entry
		},

		{ // no conflict, but with new entries
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
			4, // terms with those extra entries, it returns the index of first new entry
		},

		{ // no conflict, but with new entries
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
			4, // terms with those extra entries, it returns the index of first new entry
		},

		{ // conflicts with existing entries
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			[]raftpb.Entry{{Index: 1, Term: 4}, {Index: 2, Term: 4}}, // same index but different term
			1,
		},

		{ // conflicts with existing entries
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			[]raftpb.Entry{{Index: 2, Term: 1}, {Index: 3, Term: 3}}, // same index but different term
			2,
		},

		{ // conflicts with existing entries
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			[]raftpb.Entry{{Index: 2, Term: 1}, {Index: 3, Term: 4}, {Index: 4, Term: 4}}, // same index but different term
			2,
		},

		{ // conflicts with existing entries
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			[]raftpb.Entry{{Index: 3, Term: 1}, {Index: 4, Term: 2}, {Index: 5, Term: 4}}, // same index but different term
			3,
		},
	}
	for i, tt := range tests {
		rg := newStorageRaftLog(NewStorageStableInMemory())
		rg.appendToStorageUnstable(tt.entriesStorageUnstableToAppend...)

		if cidx := rg.findConflict(tt.entriesToCompare...); cidx != tt.firstConflictingEntryIndex {
			t.Fatalf("#%d: conflicting entry index expected %d, got %d", i, tt.firstConflictingEntryIndex, cidx)
		}
	}
}

func Test_storageRaftLog_maybeAppend(t *testing.T) { // (etcd raft TestLogMaybeAppend)
	tests := []struct {
		entriesStorageUnstableToAppend []raftpb.Entry
		indexToCommitForUnstable       uint64

		index, term, indexToCommitForMaybeAppend uint64
		entriesToMaybeAppend                     []raftpb.Entry

		wLastNewIndex   uint64
		wAppended       bool
		wCommittedIndex uint64

		entriesStorageUnstableAfterMaybeAppend []raftpb.Entry
		toPanic                                bool
	}{
		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			3, 3,
			3, // index to commit
			nil,

			3, // lastNewIndex := index + uint64(len(entries))
			true,
			3, // committed index

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			false,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			3, 3,
			4, // index to commit is 4, but lastNewIndex will still be 3 without new entries
			nil,

			3, // lastNewIndex := index + uint64(len(entries))
			true,
			3, // committed index does not grow bigger than last new index

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			false,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			3, 3,
			2, // index to commit
			nil,

			3, // lastNewIndex := index + uint64(len(entries))
			true,
			2, // committed index

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			false,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			3, 3,
			0, // index to commit
			nil,

			3, // lastNewIndex := index + uint64(len(entries))
			true,
			1, // committed index never decreases

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			false,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			3, 2, // term is not matching
			3, // index to commit
			[]raftpb.Entry{{Index: 4, Term: 4}},

			0, // lastNewIndex := index + uint64(len(entries))
			false,
			1, // committed index

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			false,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			4, 3, // index out of bound
			3, // index to commit
			[]raftpb.Entry{{Index: 5, Term: 4}},

			0, // lastNewIndex := index + uint64(len(entries))
			false,
			1, // committed index

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			false,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			0, 0,
			3, // index to commit
			nil,

			0, // lastNewIndex := index + uint64(len(entries))
			true,
			1, // committed index

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			false,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			3, 3,
			3, // index to commit
			[]raftpb.Entry{{Index: 4, Term: 4}},

			4, // lastNewIndex := index + uint64(len(entries))
			true,
			3, // committed index

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}, {Index: 4, Term: 4}},
			false,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			3, 3,
			4, // index to commit
			[]raftpb.Entry{{Index: 4, Term: 4}},

			4, // lastNewIndex := index + uint64(len(entries))
			true,
			4, // committed index

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}, {Index: 4, Term: 4}},
			false,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			3, 3,
			5, // index to commit
			[]raftpb.Entry{{Index: 4, Term: 4}},

			4, // lastNewIndex := index + uint64(len(entries))
			true,
			4, // committed index does not grow bigger than last new index

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}, {Index: 4, Term: 4}},
			false,
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			3, 3,
			5, // index to commit
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 4}},

			5, // lastNewIndex := index + uint64(len(entries))
			true,
			5, // committed index

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 4}},
			false,
		},

		{ // with conflicting entries to maybeAppend
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			2, 2,
			3, // index to commit
			[]raftpb.Entry{{Index: 3, Term: 4}}, // conflicting, so it will truncate

			3, // lastNewIndex := index + uint64(len(entries))
			true,
			3, // committed index

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 4}},
			false,
		},

		{ // with conflicting entries to maybeAppend
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			1, 1,
			3, // index to commit
			[]raftpb.Entry{{Index: 2, Term: 4}}, // conflicting, so it will truncate

			2, // lastNewIndex
			true,
			2, // committed index

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 4}},
			false,
		},

		{ // with conflicting entries to maybeAppend
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			1, 1,
			3, // index to commit
			[]raftpb.Entry{{Index: 2, Term: 4}, {Index: 3, Term: 4}}, // conflicting, so it will truncate

			3, // lastNewIndex
			true,
			3, // committed index

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 4}, {Index: 3, Term: 4}},
			false,
		},

		{ // with conflicting entries to maybeAppend
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			1, // indexToCommitForUnstable

			0, 0, // conflict with existing committed entry
			3, // index to commit
			[]raftpb.Entry{{Index: 1, Term: 4}}, // conflicting, so it will truncate

			1, // lastNewIndex
			true,
			1, // committed index

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 4}, {Index: 3, Term: 4}},
			true, // conflict with existing committed entry
		},
	}

	for i, tt := range tests {
		rg := newStorageRaftLog(NewStorageStableInMemory())
		rg.appendToStorageUnstable(tt.entriesStorageUnstableToAppend...)
		rg.commitTo(tt.indexToCommitForUnstable)

		func() {
			defer func() {
				err := recover()
				if err != nil {
					t.Logf("#%d: panic with %v", i, err)
				}

				switch {
				case err == nil && tt.toPanic:
					t.Fatalf("#%d: expected panic but didn't", i)

				case err != nil && !tt.toPanic:
					t.Fatalf("#%d: expected no panic but got panic error (%v)", i, err)
				}
			}()

			// maybeAppend returns the last index of new entries and true, if successful.
			// Otherwise, it returns 0 and false.
			lastNewIndex, appended := rg.maybeAppend(tt.index, tt.term, tt.indexToCommitForMaybeAppend, tt.entriesToMaybeAppend...)
			newCommittedIndex := rg.committedIndex

			if lastNewIndex != tt.wLastNewIndex {
				t.Fatalf("#%d: last new index expected %d, got %d", i, tt.wLastNewIndex, lastNewIndex)
			}
			if appended != tt.wAppended {
				t.Fatalf("#%d: appended expected %v, got %v", i, tt.wAppended, appended)
			}
			if newCommittedIndex != tt.wCommittedIndex {
				t.Fatalf("#%d: committed index expected %d, got %d", i, tt.wCommittedIndex, newCommittedIndex)
			}
			if !reflect.DeepEqual(tt.entriesStorageUnstableAfterMaybeAppend, rg.unstableEntries()) {
				t.Fatalf("#%d: unstable entries expected %+v, got %+v", i, rg.unstableEntries(), tt.entriesStorageUnstableAfterMaybeAppend)
			}

			if appended && len(tt.entriesToMaybeAppend) > 0 {
				ents, err := rg.slice(rg.lastIndex()-uint64(len(tt.entriesToMaybeAppend))+1, rg.lastIndex()+1, math.MaxUint64)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(tt.entriesToMaybeAppend, ents) {
					t.Fatalf("#%d: appended entries expected %+v, got %+v", i, tt.entriesToMaybeAppend, ents)
				}
			}
		}()
	}
}

func Test_storageRaftLog_term(t *testing.T) { // (etcd raft TestTerm)
	var (
		indexOffset uint64 = 100
		num         uint64 = 300
	)
	ms := NewStorageStableInMemory()
	ms.ApplySnapshot(raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{Index: indexOffset, Term: 1},
	})
	rg := newStorageRaftLog(ms)

	for i := uint64(1); i < num; i++ {
		rg.appendToStorageUnstable(raftpb.Entry{Index: indexOffset + i, Term: i})
	}

	tests := []struct {
		indexToGetTerm uint64

		wterm uint64
	}{
		{indexOffset, 1},
		{indexOffset - 1, 0},
		{indexOffset + num, 0},

		{indexOffset + num, 0},
		{indexOffset + num - 1, num - 1},
		{indexOffset + num/2, num / 2},
	}

	for i, tt := range tests {
		term, err := rg.term(tt.indexToGetTerm)
		if err != nil {
			t.Fatal(err)
		}

		if term != tt.wterm {
			t.Fatalf("#%d: term expected %d, got %d", i, tt.wterm, term)
		}
	}
}

func Test_storageRaftLog_term_restoreSnapshot(t *testing.T) { // (etcd raft TestTermWithUnstableSnapshot)
	var (
		snapshotIndex         uint64 = 100
		unstableSnapshotIndex        = snapshotIndex + 5
	)
	ms := NewStorageStableInMemory()
	ms.ApplySnapshot(raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: snapshotIndex, Term: 1}})

	rg := newStorageRaftLog(ms)
	rg.restoreSnapshot(raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: unstableSnapshotIndex, Term: 1}})
	// dummyIndex (first entry index) is now 105

	tests := []struct {
		index uint64
		wterm uint64
	}{
		{snapshotIndex, 0},     // 0, if "index < dummyIndex || rg.lastIndex() < index"
		{snapshotIndex + 1, 0}, // 0, if "index < dummyIndex || rg.lastIndex() < index"
		{snapshotIndex - 1, 0}, // 0, if "index < dummyIndex || rg.lastIndex() < index"
		{unstableSnapshotIndex, 1},
	}
	for i, tt := range tests {
		term, err := rg.term(tt.index)
		if err != nil {
			t.Fatalf("#%d: term error (%v)", i, err)
		}
		if term != tt.wterm {
			t.Fatalf("#%d: term expected %d, got %d", i, tt.wterm, term)
		}
	}
}

func Test_storageRaftLog_persistedEntriesAt(t *testing.T) { // (etcd raft TestStableTo)
	tests := []struct {
		entriesToAppendToStorageUnstable []raftpb.Entry

		indexToPersist uint64
		termToPersist  uint64

		storageUnstableIndexOffsetAfterPersist uint64
		storageUnstableEntriesAfterPersist     []raftpb.Entry
	}{
		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},

			1, // index to persist
			1, // term  to persist

			2, // unstable entries index offset
			[]raftpb.Entry{{Index: 2, Term: 2}}, // unstable entries after append, persist
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},

			2, // index to persist
			2, // term  to persist

			3,   // unstable entries index offset
			nil, // unstable entries after append, persist
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},

			3, // index to persist (bad index)
			1, // term  to persist

			1, // unstable entries index offset
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}}, // unstable entries after append, persist
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},

			2, // index to persist
			3, // term  to persist (bad term)

			1, // unstable entries index offset (bad term, so offset doesn't change)
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}}, // unstable entries after append, persist
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},

			2, // index to persist
			1, // term  to persist (bad term)

			1, // unstable entries index offset (bad term, so offset doesn't change)
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}}, // unstable entries after append, persist
		},
	}

	for i, tt := range tests {
		rg := newStorageRaftLog(NewStorageStableInMemory())

		// appendToStorageUnstable will truncate and append
		rg.appendToStorageUnstable(tt.entriesToAppendToStorageUnstable...)

		// only update unstable entries if term is matched with an unstable entry
		// Then, su.indexOffset = index + 1
		rg.persistedEntriesAt(tt.indexToPersist, tt.termToPersist)

		// after persist
		if rg.storageUnstable.indexOffset != tt.storageUnstableIndexOffsetAfterPersist {
			t.Fatalf("#%d: unstable storage index offset expected %d, got %d", i, tt.storageUnstableIndexOffsetAfterPersist, rg.storageUnstable.indexOffset)
		}
		if !reflect.DeepEqual(rg.unstableEntries(), tt.storageUnstableEntriesAfterPersist) {
			t.Fatalf("#%d: unstable storage entries expected %+v, got %+v", i, rg.unstableEntries(), tt.storageUnstableEntriesAfterPersist)
		}
	}
}

func Test_storageRaftLog_persistedSnapshotAt(t *testing.T) { // (etcd raft TestStableToWithSnap)
	tests := []struct {
		snapshotToApply                  raftpb.Snapshot
		entriesToAppendToStorageUnstable []raftpb.Entry

		indexToPersist uint64
		termToPersist  uint64

		storageUnstableIndexOffsetAfterPersist uint64
		storageUnstableEntriesAfterPersist     []raftpb.Entry
	}{
		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 5, Term: 2}},
			nil, // to append

			4, 2, // to persist

			6,   // indexOffset after append
			nil, // entries after append, persists
		},
		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 5, Term: 2}},
			nil, // to append

			5, 2, // to persist

			6,   // indexOffset after append
			nil, // entries after append, persists
		},
		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 5, Term: 2}},
			nil, // to append

			6, 2, // to persist

			6,   // indexOffset after append
			nil, // entries after append, persists
		},

		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 5, Term: 2}},
			nil, // to append

			4, 3, // to persist

			6,   // indexOffset after append
			nil, // entries after append, persists
		},
		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 5, Term: 2}},
			nil, // to append

			5, 3, // to persist

			6,   // indexOffset after append
			nil, // entries after append, persists
		},
		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 5, Term: 2}},
			nil, // to append

			6, 3, // to persist

			6,   // indexOffset after append
			nil, // entries after append, persists
		},

		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 5, Term: 2}},
			[]raftpb.Entry{{Index: 6, Term: 2}}, // to append

			4, 2, // to persist

			6, // indexOffset after append
			[]raftpb.Entry{{Index: 6, Term: 2}}, // entries after append, persists
		},
		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 5, Term: 2}},
			[]raftpb.Entry{{Index: 6, Term: 2}}, // to append

			5, 2, // to persist

			6, // indexOffset after append
			[]raftpb.Entry{{Index: 6, Term: 2}}, // entries after append, persists
		},
		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 5, Term: 2}},
			[]raftpb.Entry{{Index: 6, Term: 2}}, // to append

			6, 2, // to persist

			7,   // indexOffset after append
			nil, // entries after append, persists
		},

		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 5, Term: 2}},
			[]raftpb.Entry{{Index: 6, Term: 2}}, // to append

			4, 3, // to persist

			6, // indexOffset after append
			[]raftpb.Entry{{Index: 6, Term: 2}}, // entries after append, persists
		},
		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 5, Term: 2}},
			[]raftpb.Entry{{Index: 6, Term: 2}}, // to append

			5, 3, // to persist

			6, // indexOffset after append
			[]raftpb.Entry{{Index: 6, Term: 2}}, // entries after append, persists
		},
		{
			raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 5, Term: 2}},
			[]raftpb.Entry{{Index: 6, Term: 2}}, // to append

			6, 3, // to persist

			6, // indexOffset after append
			[]raftpb.Entry{{Index: 6, Term: 2}}, // entries after append, persists
		},
	}

	for i, tt := range tests {
		ms := NewStorageStableInMemory()
		ms.ApplySnapshot(tt.snapshotToApply)

		rg := newStorageRaftLog(ms)

		// appendToStorageUnstable will truncate and append
		rg.appendToStorageUnstable(tt.entriesToAppendToStorageUnstable...)

		// only update unstable entries if term is matched with an unstable entry
		// Then, su.indexOffset = index + 1
		rg.persistedEntriesAt(tt.indexToPersist, tt.termToPersist)

		// after persist
		if rg.storageUnstable.indexOffset != tt.storageUnstableIndexOffsetAfterPersist {
			t.Fatalf("#%d: unstable storage index offset expected %d, got %d", i, tt.storageUnstableIndexOffsetAfterPersist, rg.storageUnstable.indexOffset)
		}
		if !reflect.DeepEqual(rg.unstableEntries(), tt.storageUnstableEntriesAfterPersist) {
			t.Fatalf("#%d: unstable storage entries expected %+v, got %+v", i, rg.unstableEntries(), tt.storageUnstableEntriesAfterPersist)
		}
	}
}

func Test_storageRaftLog_commitTo(t *testing.T) { // (etcd raft TestCommitTo)
	tests := []struct {
		entriesToAppendToStorageUnstable []raftpb.Entry
		initialCommittedIndex            uint64

		indexToCommit   uint64
		wCommittedIndex uint64

		storageUnstableIndexOffsetAfterCommitTo uint64
		storageUnstableEntriesAfterCommitTo     []raftpb.Entry
		toPanic                                 bool
	}{
		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}}, // to append
			2,

			3, // index to commit
			3, // expected committedIndex afterwards

			1, // indexOffset after commit
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}}, // entries after commit
			false, // to panic
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}}, // to append
			2,

			1, // index to commit
			2, // expected committedIndex afterwards (ensure that commitTo never decreases the committed index)

			1, // indexOffset after commit
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}}, // entries after commit
			false, // to panic
		},

		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}}, // to append
			2,

			4, // index to commit (out of range "rg.lastIndex() < indexToCommit", so panic)
			0, // expected committedIndex afterwards

			1, // indexOffset after commit
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}}, // entries after commit
			true, // to panic
		},
	}

	for i, tt := range tests {
		rg := newStorageRaftLog(NewStorageStableInMemory())
		rg.appendToStorageUnstable(tt.entriesToAppendToStorageUnstable...) // appendToStorageUnstable will truncate and append
		rg.commitTo(tt.initialCommittedIndex)

		func() {
			defer func() {
				err := recover()
				if err != nil {
					t.Logf("#%d: panic with %v", i, err)
				}

				switch {
				case err == nil && tt.toPanic:
					t.Fatalf("#%d: expected panic but didn't", i)

				case err != nil && !tt.toPanic:
					t.Fatalf("#%d: expected no panic but got panic error (%v)", i, err)
				}
			}()

			rg.commitTo(tt.indexToCommit)

			// after commitTo
			if rg.committedIndex != tt.wCommittedIndex {
				t.Fatalf("#%d: committed index expected %d, got %d", i, tt.wCommittedIndex, rg.committedIndex)
			}
			if rg.storageUnstable.indexOffset != tt.storageUnstableIndexOffsetAfterCommitTo {
				t.Fatalf("#%d: unstable storage index offset expected %d, got %d", i, tt.storageUnstableIndexOffsetAfterCommitTo, rg.storageUnstable.indexOffset)
			}
			if !reflect.DeepEqual(rg.unstableEntries(), tt.storageUnstableEntriesAfterCommitTo) {
				t.Fatalf("#%d: unstable storage entries expected %+v, got %+v", i, rg.unstableEntries(), tt.storageUnstableEntriesAfterCommitTo)
			}
		}()
	}
}

func Test_storageRaftLog_maybeCommit_appliedTo_Compact(t *testing.T) { // (etcd raft TestCompaction)
	var entriesToAppendToStorageStable []raftpb.Entry
	for i := uint64(1); i <= 1000; i++ {
		entriesToAppendToStorageStable = append(entriesToAppendToStorageStable, raftpb.Entry{Index: i, Term: i})
	}

	tests := []struct {
		entriesToAppendToStorageStable []raftpb.Entry

		indexToMaybeCommit uint64
		termToMaybeCommit  uint64

		wCommittedIndex uint64

		indexToApply uint64

		indexToCompact                     uint64
		wErrorAfterCompact                 error
		numberOfLeftEntriesInStorageStable int
		toPanic                            bool
	}{
		{
			entriesToAppendToStorageStable, // to append to stable storage

			1000, // index to commit
			1000, // term to commit
			1000, // expected committedIndex
			1000, // index to apply

			300, // index to compact on
			nil,
			700,
			false, // to panic
		},

		{
			entriesToAppendToStorageStable, // to append to stable storage

			1000, // index to commit
			1000, // term to commit
			1000, // expected committedIndex
			1000, // index to apply

			800, // index to compact on
			nil,
			200,
			false, // to panic
		},

		{
			entriesToAppendToStorageStable, // to append to stable storage

			1000, // index to commit
			1000, // term to commit
			1000, // expected committedIndex
			1000, // index to apply

			1001, // index to compact on (out of bound with "compactIndex > ms.lastIndex()", so panic)
			nil,
			0,
			true, // to panic
		},
	}

	for i, tt := range tests {
		ms := NewStorageStableInMemory()
		ms.Append(tt.entriesToAppendToStorageStable...)

		rg := newStorageRaftLog(ms)

		// maybeCommit is only successful if 'indexToCommit' is greater than current 'committedIndex'
		// and the current term of 'indexToCommit' matches the 'termToCommit', without ErrCompacted.
		rg.maybeCommit(tt.indexToMaybeCommit, tt.termToMaybeCommit)

		if rg.committedIndex != tt.wCommittedIndex {
			t.Fatalf("#%d: committed index expected %d, got %d", i, tt.wCommittedIndex, rg.committedIndex)
		}

		rg.appliedTo(tt.indexToApply)

		func() {
			defer func() {
				err := recover()
				if err != nil {
					t.Logf("#%d: panic with %v", i, err)
				}

				switch {
				case err == nil && tt.toPanic:
					t.Fatalf("#%d: expected panic but didn't", i)

				case err != nil && !tt.toPanic:
					t.Fatalf("#%d: expected no panic but got panic error (%v)", i, err)
				}
			}()

			// panic if "compactIndex > ms.lastIndex()"
			err := ms.Compact(tt.indexToCompact)
			if err != tt.wErrorAfterCompact {
				t.Fatalf("#%d: compact error expected %v, got %v", i, tt.wErrorAfterCompact, err)
			}

			// after compact
			if len(rg.allEntries()) != tt.numberOfLeftEntriesInStorageStable {
				t.Fatalf("#%d: number of left entires expected %d, got %d", i, tt.numberOfLeftEntriesInStorageStable, len(rg.allEntries()))
			}

			err = ms.Compact(tt.indexToCompact)
			if err != ErrCompacted {
				t.Fatalf("#%d: second compaction error expected %v, got %v", i, ErrCompacted, err)
			}
		}()
	}
}

func Test_storageRaftLog_maybeCommit_appliedTo_Compact_manual(t *testing.T) { // (etcd raft TestCompactionSideEffects)
	ms := NewStorageStableInMemory()
	for i := uint64(1); i <= 1000; i++ {
		ms.Append(raftpb.Entry{Index: i, Term: i})
	}

	rg := newStorageRaftLog(ms)

	if ok := rg.maybeCommit(2000, 2000); ok {
		t.Fatal("maybeCommit must have failed")
	}

	if ok := rg.maybeCommit(1000, 1000); !ok {
		t.Fatal("maybeCommit failed")
	}
	if rg.committedIndex != 1000 {
		t.Fatalf("committed index expected 1000, got %d", rg.committedIndex)
	}

	rg.appliedTo(1000)
	if rg.appliedIndex != 1000 {
		t.Fatalf("applied index expected 1000, got %d", rg.appliedIndex)
	}

	if rg.lastIndex() != 1000 {
		t.Fatalf("last index expected 1000, got %d", rg.lastIndex())
	}

	// append more unstable entries
	for i := uint64(1001); i <= 1500; i++ {
		rg.appendToStorageUnstable(raftpb.Entry{Index: i, Term: i})
	}

	if rg.storageUnstable.indexOffset != 1001 {
		t.Fatalf("unstable storage offset expected %d, got %d", 1001, rg.storageUnstable.indexOffset)
	}

	// match all the index and terms
	for i := uint64(1); i <= 1500; i++ {
		tm, err := rg.term(i)
		if err != nil {
			t.Fatal(err)
		}
		if tm != i {
			t.Fatalf("term expected %d, got %d", i, tm)
		}
		if !rg.matchTerm(i, i) {
			t.Fatalf("term doesn't match for index %d, term %d", i, i)
		}
	}

	unstableEntries := rg.unstableEntries()
	if len(unstableEntries) != 500 {
		t.Fatalf("len(unstableEntries) expected 500, got %d", len(unstableEntries))
	}
	if unstableEntries[0].Index != 1001 {
		t.Fatalf("unstableEntries[0].Index expected 1001, got %d", unstableEntries[0].Index)
	}
	if unstableEntries[0].Term != 1001 {
		t.Fatalf("unstableEntries[0].Term expected 1001, got %d", unstableEntries[0].Term)
	}

	// append one more entry
	rg.appendToStorageUnstable(raftpb.Entry{Index: 1501, Term: 1501})
	if rg.lastIndex() != 1501 {
		t.Fatalf("last index expected 1501, got %d", rg.lastIndex())
	}

	ents, err := rg.entries(1500, math.MaxUint64)
	if err != nil {
		t.Fatal(err)
	}
	wents := []raftpb.Entry{{Index: 1500, Term: 1500}, {Index: 1501, Term: 1501}}
	if !reflect.DeepEqual(ents, wents) {
		t.Fatalf("entries expected %+v, got %+v", wents, ents)
	}

	// compact at 900
	err = ms.Compact(900)
	if err != nil {
		t.Fatal(err)
	}
	if rg.lastIndex() != 1501 {
		t.Fatalf("last index expected 1501, got %d", rg.lastIndex())
	}
	for i := uint64(900); i <= 1501; i++ {
		tm, err := rg.term(i)
		if err != nil {
			t.Fatal(err)
		}
		if tm != i {
			t.Fatalf("term expected %d, got %d", i, tm)
		}
		if !rg.matchTerm(i, i) {
			t.Fatalf("term doesn't match for index %d, term %d", i, i)
		}
	}
}
