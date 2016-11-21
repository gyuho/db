package raft

import (
	"math"
	"reflect"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestStorageFirstIndex)
func Test_StorageStableInMemory_FirstIndex(t *testing.T) {
	snapshotEntries := []raftpb.Entry{
		{Index: 3, Term: 3},
		{Index: 4, Term: 4},
		{Index: 5, Term: 5},
	}
	st := &StorageStableInMemory{snapshotEntries: snapshotEntries}

	firstIndex, err := st.FirstIndex()
	if err != nil {
		t.Fatal(err)
	}
	if firstIndex != 4 {
		t.Fatalf("last index expected 4, got %d", firstIndex)
	}

	// compact up to index 4
	if err = st.Compact(4); err != nil {
		t.Fatal(err)
	}

	firstIndex, err = st.FirstIndex()
	if err != nil {
		t.Fatal(err)
	}
	if firstIndex != 5 {
		t.Fatalf("last index expected 5, got %d", firstIndex)
	}
}

// (etcd raft.TestStorageLastIndex)
func Test_StorageStableInMemory_LastIndex(t *testing.T) {
	snapshotEntries := []raftpb.Entry{
		{Index: 3, Term: 3},
		{Index: 4, Term: 4},
		{Index: 5, Term: 5},
	}
	st := &StorageStableInMemory{snapshotEntries: snapshotEntries}

	lastIndex, err := st.LastIndex()
	if err != nil {
		t.Fatal(err)
	}
	if lastIndex != 5 {
		t.Fatalf("last index expected 5, got %d", lastIndex)
	}

	if err = st.Append([]raftpb.Entry{{Index: 6, Term: 5}}...); err != nil {
		t.Fatal(err)
	}
	lastIndex, err = st.LastIndex()
	if err != nil {
		t.Fatal(err)
	}
	if lastIndex != 6 {
		t.Fatalf("last index expected 6, got %d", lastIndex)
	}
}

// (etcd raft.TestStorageTerm)
func Test_StorageStableInMemory_Term(t *testing.T) {
	snapshotEntries := []raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}}
	tests := []struct {
		entryIdx uint64

		werr    error
		wterm   uint64
		toPanic bool
	}{
		{2, ErrCompacted, 0, false},
		{3, nil, 3, false},
		{4, nil, 4, false},
		{5, nil, 5, false},
		{6, ErrUnavailable, 0, false},
	}

	for i, tt := range tests {
		st := &StorageStableInMemory{snapshotEntries: snapshotEntries}

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

			term, err := st.Term(tt.entryIdx)
			if err != tt.werr {
				t.Fatalf("#%d: error expected %v, got %v", i, tt.werr, err)
			}
			if term != tt.wterm {
				t.Fatalf("#%d: term expected %d, got %d", i, tt.wterm, term)
			}
		}()
	}
}

// (etcd raft.TestStorageEntries)
func Test_StorageStableInMemory_Entries(t *testing.T) {
	snapshotEntries := []raftpb.Entry{
		{Index: 3, Term: 3},
		{Index: 4, Term: 4},
		{Index: 5, Term: 5},
		{Index: 6, Term: 6},
	}

	tests := []struct {
		// Entries returns [startIdx, endIdx)
		startIdx, endIdx, limitSize uint64

		werr     error
		wentries []raftpb.Entry
	}{
		{
			2, 6, math.MaxUint64,
			ErrCompacted,
			nil,
		},

		{
			3, 4, math.MaxUint64,
			ErrCompacted,
			nil,
		},

		{
			4, 5, math.MaxUint64,
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}},
		},

		{
			4, 6, math.MaxUint64,
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},

		{
			4, 7, math.MaxUint64,
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 6}},
		},

		// even limitSize 0 should return at least one entry
		{
			4, 7, 0,
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}},
		},

		// limit to 2 entries
		{
			4, 7, uint64(snapshotEntries[1].Size() + snapshotEntries[2].Size()),
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},

		// limit to 2 entries
		{
			4, 7, uint64(snapshotEntries[1].Size() + snapshotEntries[2].Size() + snapshotEntries[3].Size()/2),
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},

		// limit to 2 entries
		{
			4, 7, uint64(snapshotEntries[1].Size() + snapshotEntries[2].Size() + snapshotEntries[3].Size() - 1),
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},

		// limit to 3 entries
		{
			4, 7, uint64(snapshotEntries[1].Size() + snapshotEntries[2].Size() + snapshotEntries[3].Size()),
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 6}},
		},
	}

	for i, tt := range tests {
		st := &StorageStableInMemory{snapshotEntries: snapshotEntries}
		entries, err := st.Entries(tt.startIdx, tt.endIdx, tt.limitSize)
		if err != tt.werr {
			t.Fatalf("#%d: error expected %v, got %v", i, tt.werr, err)
		}
		if !reflect.DeepEqual(entries, tt.wentries) {
			t.Fatalf("#%d: entries expected %+v, got %+v", i, tt.wentries, entries)
		}
	}
}

// (etcd raft.TestStorageAppend)
func Test_StorageStableInMemory_Append(t *testing.T) {
	tests := []struct {
		snapshotEntries []raftpb.Entry
		entriesToAppend []raftpb.Entry

		werr     error
		wentries []raftpb.Entry
	}{
		{ // append
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
			[]raftpb.Entry{{Index: 6, Term: 5}},
			nil,
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 5}},
		},

		{ // ignore duplicate entries
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
			nil,
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},

		{ // overwrite terms with newly appended entries
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 6}, {Index: 5, Term: 6}},
			nil,
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 6}, {Index: 5, Term: 6}},
		},

		{ // overwrite with newly appended entries
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 5}},
			nil,
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 5}},
		},

		{ // truncate incoming entries for compacted snapshot, truncate existing ones with appended ones
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
			[]raftpb.Entry{{Index: 2, Term: 3}, {Index: 3, Term: 3}, {Index: 4, Term: 5}},
			//                   ignored         -----append from here----->
			nil,
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 5}},
		},

		{ // truncate existing entries with appended ones
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
			[]raftpb.Entry{{Index: 4, Term: 5}}, // this will truncate existing entries by index
			nil,
			[]raftpb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 5}},
		},
	}

	for i, tt := range tests {
		original := make([]raftpb.Entry, len(tt.snapshotEntries))
		copy(original, tt.snapshotEntries)

		st := &StorageStableInMemory{snapshotEntries: tt.snapshotEntries}
		if err := st.Append(tt.entriesToAppend...); err != tt.werr {
			t.Fatalf("#%d: error expected %v, got %v", i, tt.werr, err)
		}

		if !reflect.DeepEqual(st.snapshotEntries, tt.wentries) {
			t.Fatalf("#%d: snapshot entries expected %+v, got %+v", i, tt.wentries, st.snapshotEntries)
		}

		// make sure 'Append' did not manipulate the original entries
		if !reflect.DeepEqual(original, tt.snapshotEntries) {
			t.Fatalf("#%d: original snapshot entries expected %+v, got %+v", i, tt.snapshotEntries, original)
		}
	}
}

// (etcd raft.TestStorageCreateSnapshot)
func Test_StorageStableInMemory_CreateSnapshot(t *testing.T) {
	var (
		snapshotEntries = []raftpb.Entry{
			{Index: 3, Term: 3},
			{Index: 4, Term: 4},
			{Index: 5, Term: 5},
		}
		cs   = raftpb.ConfigState{IDs: []uint64{100, 200, 300}}
		data = []byte("data")
	)

	tests := []struct {
		snapshotIndex uint64

		werr  error
		wsnap raftpb.Snapshot
	}{
		{4, nil, raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 4, ConfigState: cs}, Data: data}},
		{5, nil, raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 5, Term: 5, ConfigState: cs}, Data: data}},
	}

	for i, tt := range tests {
		st := &StorageStableInMemory{snapshotEntries: snapshotEntries}
		snap, err := st.CreateSnapshot(tt.snapshotIndex, &cs, data)
		if err != tt.werr {
			t.Fatalf("#%d: error expected %v, got %v", i, tt.werr, err)
		}
		if !reflect.DeepEqual(snap, tt.wsnap) {
			t.Fatalf("#%d: snap expected %+v, got %+v", i, tt.wsnap, snap)
		}
	}
}

// (etcd raft.TestStorageCompact)
func Test_StorageStableInMemory_Compact(t *testing.T) {
	snapshotEntries := []raftpb.Entry{
		{Index: 3, Term: 3},
		{Index: 4, Term: 4},
		{Index: 5, Term: 5},
	}
	tests := []struct {
		compactIndex uint64

		werr                 error
		wFirstIndexInStorage uint64
		wFirstTermInStorage  uint64
		wSnapshotEntriesN    int
	}{
		{2, ErrCompacted, 3, 3, 3},
		{3, ErrCompacted, 3, 3, 3},
		{4, nil, 4, 4, 2},
		{5, nil, 5, 5, 1},
	}

	for i, tt := range tests {
		st := &StorageStableInMemory{snapshotEntries: snapshotEntries}
		if err := st.Compact(tt.compactIndex); err != tt.werr {
			t.Fatalf("#%d: error expected %v, got %v", i, tt.werr, err)
		}

		if st.snapshotEntries[0].Index != tt.wFirstIndexInStorage {
			t.Fatalf("#%d: first index in storage expected %d, got %d", i, tt.wFirstIndexInStorage, st.snapshotEntries[0].Index)
		}

		if st.snapshotEntries[0].Term != tt.wFirstTermInStorage {
			t.Fatalf("#%d: first term in storage expected %d, got %d", i, tt.wFirstTermInStorage, st.snapshotEntries[0].Term)
		}

		if len(st.snapshotEntries) != tt.wSnapshotEntriesN {
			t.Fatalf("#%d: size of snapshot entries expected %d, got %d", i, tt.wSnapshotEntriesN, len(st.snapshotEntries))
		}
	}
}

// (etcd raft.TestStorageApplySnapshot)
func Test_StorageStableInMemory_ApplySnapshot_OutOfDate(t *testing.T) {
	snapshots := []raftpb.Snapshot{
		{
			Data: []byte("testdata"),
			Metadata: raftpb.SnapshotMetadata{
				Index: 4, Term: 4,
				ConfigState: raftpb.ConfigState{IDs: []uint64{1, 2, 3}},
			},
		},
		{
			Data: []byte("testdata"),
			Metadata: raftpb.SnapshotMetadata{
				Index: 3, Term: 3,
				ConfigState: raftpb.ConfigState{IDs: []uint64{1, 2, 3}},
			},
		},
	}

	st := NewStorageStableInMemory()

	if err := st.ApplySnapshot(snapshots[0]); err != nil {
		t.Fatal(err)
	}

	if err := st.ApplySnapshot(snapshots[1]); err != ErrSnapOutOfDate {
		t.Fatal(err)
	}
}
