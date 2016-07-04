package raft

import (
	"math"
	"reflect"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

func Test_raftLog(t *testing.T) { // (etcd raft TestLogRestore)
	var (
		index    uint64 = 1000
		term     uint64 = 1000
		snapshot        = raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: index, Term: term}}
	)
	ms := NewStorageStableInMemory()
	ms.SetSnapshot(snapshot)
	rg := newRaftLog(ms)

	if entN := len(rg.allEntries()); entN != 0 {
		t.Fatalf("entry number expected 0, got %d", entN)
	}

	if fidx := rg.firstIndex(); fidx != index+1 { // get snapshot index + 1
		t.Fatalf("first index expected %d, got %d", index+1, fidx)
	}

	if rg.committedIndex != index { // newRaftLog sets this
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

func Test_raftLog_mustCheckOutOfBounds(t *testing.T) { // (etcd raft TestIsOutOfBounds)
	var (
		indexOffset uint64 = 100
		snapshot           = raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: indexOffset}}
	)
	ms := NewStorageStableInMemory()
	ms.SetSnapshot(snapshot)
	rg := newRaftLog(ms)

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

func Test_raftLog_slice(t *testing.T) { // (etcd raft TestSlice)
	var (
		indexOffset uint64 = 100
		snapshot           = raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: indexOffset}}
		num         uint64 = 100
		midIndex           = indexOffset + num/2 // 150
		lastIndex          = indexOffset + num   // 200

		midIndexEntry = raftpb.Entry{Index: midIndex, Term: midIndex}
	)
	ms := NewStorageStableInMemory()
	ms.SetSnapshot(snapshot)
	for i := uint64(1); i < num/2; i++ {
		ms.Append(raftpb.Entry{Index: indexOffset + i, Term: indexOffset + i})
	}
	rg := newRaftLog(ms)

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

func Test_raftLog_unstableEntries(t *testing.T) { // (etcd raft TestUnstableEnts)

}

func Test_raftLog_hasNextEntriesToApply(t *testing.T) { // (etcd raft TestHasNextEnts)

}

func Test_raftLog_nextEntriesToApply(t *testing.T) { // (etcd raft TestNextEnts)

}

func Test_raftLog_isUpToDate(t *testing.T) { // (etcd raft TestIsUpToDate)

}

func Test_raftLog_appendToStorageUnstable(t *testing.T) { // (etcd raft TestAppend)
	existingEntries := []raftpb.Entry{
		{Index: 1, Term: 1},
		{Index: 2, Term: 2},
	}

	tests := []struct {
		entriesToAppend []raftpb.Entry

		wNewIndexAfterAppend    uint64
		wNewLogEntries          []raftpb.Entry
		wNewUnstableIndexOffset uint64
	}{
		{
			[]raftpb.Entry{},

			2,
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},
			3, // because newRaftLog makes "rg.storageUnstable.indexOffset = lastIndex + 1"
		},

		{
			[]raftpb.Entry{{Index: 3, Term: 2}},

			3,
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 2}},
			3, // because newRaftLog makes "rg.storageUnstable.indexOffset = lastIndex + 1"
		},

		{ // conflicting entry at index 1
			[]raftpb.Entry{{Index: 1, Term: 2}},

			1,
			[]raftpb.Entry{{Index: 1, Term: 2}},
			1, // because truncateAndAppend makes "su.indexOffset = firstIndexInEntriesToAppend"
		},

		{ // conflicting entry at index 2
			[]raftpb.Entry{{Index: 2, Term: 3}, {Index: 3, Term: 3}},

			3,
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 3}, {Index: 3, Term: 3}},
			2, // because truncateAndAppend makes "su.indexOffset = firstIndexInEntriesToAppend"
		},
	}

	for i, tt := range tests {
		ms := NewStorageStableInMemory()
		ms.Append(existingEntries...)
		rg := newRaftLog(ms)

		nindex := rg.appendToStorageUnstable(tt.entriesToAppend...)
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

func Test_raftLog_findConflictingTerm(t *testing.T) { // (etcd raft TestFindConflict)

}

func Test_raftLog_maybeAppend(t *testing.T) { // (etcd raft TestLogMaybeAppend)

}

func Test_raftLog_term(t *testing.T) { // (etcd raft TestTerm)
	var (
		indexOffset uint64 = 100
		num         uint64 = 300
	)
	ms := NewStorageStableInMemory()
	ms.SetSnapshot(raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{Index: indexOffset, Term: 1},
	})
	rg := newRaftLog(ms)

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

func Test_raftLog_term_UnstableSnapshot(t *testing.T) { // (etcd raft TestTermWithUnstableSnapshot)

}

func Test_raftLog_persistedEntriesAt(t *testing.T) { // (etcd raft TestStableTo)

}

func Test_raftLog_persistedSnapshotAt(t *testing.T) { // (etcd raft TestStableToWithSnap)

}

func Test_raftLog_commitTo(t *testing.T) { // (etcd raft TestCommitTo)

}

func Test_raftLog_maybeCommit_appliedTo_Compact(t *testing.T) { // (etcd raft TestCompaction)

}

func Test_raftLog_maybeCommit_appliedTo_Compact_SideEffects(t *testing.T) { // (etcd raft TestCompactionSideEffects)

}
