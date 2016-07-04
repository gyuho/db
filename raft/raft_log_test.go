package raft

import (
	"math"
	"reflect"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

func Test_raftLog(t *testing.T) { // (etcd raft TestLogRestore)
	var (
		ms              = NewStorageStableInMemory()
		index    uint64 = 1000
		term     uint64 = 1000
		snapshot        = raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: index, Term: term}}
	)
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
		ms                 = NewStorageStableInMemory()
		indexOffset uint64 = 100
		firstIndex         = indexOffset + 1

		num uint64 = 100

		snapshot = raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: indexOffset}}
	)
	ms.SetSnapshot(snapshot)
	rg := newRaftLog(ms)

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
