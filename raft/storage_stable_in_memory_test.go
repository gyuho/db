package raft

import (
	"math"
	"reflect"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

func Test_StorageStableInMemory_Term(t *testing.T) {
	ents := []raftpb.Entry{
		{Index: 3, Term: 3},
		{Index: 4, Term: 4},
		{Index: 5, Term: 5},
	}
	tests := []struct {
		entryIdx uint64

		werr  error
		wterm uint64
	}{
		{2, ErrCompacted, 0},
		{3, nil, 3},
		{4, nil, 4},
		{5, nil, 5},
	}

	for i, tt := range tests {
		st := &StorageStableInMemory{snapshotEntries: ents}
		term, err := st.Term(tt.entryIdx)
		if err != tt.werr {
			t.Fatalf("#%d: error expected %v, got %v", i, tt.werr, err)
		}
		if term != tt.wterm {
			t.Fatalf("#%d: term expected %d, got %d", i, tt.wterm, term)
		}
	}
}

func Test_StorageStableInMemory_Entries(t *testing.T) {
	ents := []raftpb.Entry{
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
			4, 7, uint64(ents[1].Size() + ents[2].Size()),
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},

		// limit to 2 entries
		{
			4, 7, uint64(ents[1].Size() + ents[2].Size() + ents[3].Size()/2),
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},

		// limit to 2 entries
		{
			4, 7, uint64(ents[1].Size() + ents[2].Size() + ents[3].Size() - 1),
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},

		// limit to 3 entries
		{
			4, 7, uint64(ents[1].Size() + ents[2].Size() + ents[3].Size()),
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 6}},
		},
	}

	for i, tt := range tests {
		st := &StorageStableInMemory{snapshotEntries: ents}
		entries, err := st.Entries(tt.startIdx, tt.endIdx, tt.limitSize)
		if err != tt.werr {
			t.Fatalf("#%d: error expected %v, got %v", i, tt.werr, err)
		}
		if !reflect.DeepEqual(entries, tt.wentries) {
			t.Fatalf("#%d: entries expected %+v, got %+v", i, tt.wentries, entries)
		}
	}
}

func Test_StorageStableInMemory_LastIndex(t *testing.T) {
	ents := []raftpb.Entry{
		{Index: 3, Term: 3},
		{Index: 4, Term: 4},
		{Index: 5, Term: 5},
	}
	st := &StorageStableInMemory{snapshotEntries: ents}

	lastIdx, err := st.LastIndex()
	if err != nil {
		t.Fatal(err)
	}
	if lastIdx != 5 {
		t.Fatalf("last index expected 5, got %d", lastIdx)
	}

	if err = st.Append([]raftpb.Entry{{Index: 6, Term: 5}}); err != nil {
		t.Fatal(err)
	}
	lastIdx, err = st.LastIndex()
	if err != nil {
		t.Fatal(err)
	}
	if lastIdx != 6 {
		t.Fatalf("last index expected 6, got %d", lastIdx)
	}
}

func Test_StorageStableInMemory_FirstIndex(t *testing.T) {
	ents := []raftpb.Entry{
		{Index: 3, Term: 3},
		{Index: 4, Term: 4},
		{Index: 5, Term: 5},
	}
	st := &StorageStableInMemory{snapshotEntries: ents}

	firstIdx, err := st.FirstIndex()
	if err != nil {
		t.Fatal(err)
	}
	if firstIdx != 4 {
		t.Fatalf("last index expected 4, got %d", firstIdx)
	}

	// compact up to index 4
	if err = st.Compact(4); err != nil {
		t.Fatal(err)
	}

	firstIdx, err = st.FirstIndex()
	if err != nil {
		t.Fatal(err)
	}
	if firstIdx != 5 {
		t.Fatalf("last index expected 5, got %d", firstIdx)
	}
}

func Test_StorageStableInMemory_Append(t *testing.T) {
	tests := []struct {
		entries         []raftpb.Entry
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
		original := make([]raftpb.Entry, len(tt.entries))
		copy(original, tt.entries)

		st := &StorageStableInMemory{snapshotEntries: tt.entries}
		if err := st.Append(tt.entriesToAppend); err != tt.werr {
			t.Fatalf("#%d: error expected %v, got %v", i, tt.werr, err)
		}

		if !reflect.DeepEqual(st.snapshotEntries, tt.wentries) {
			t.Fatalf("#%d: snapshot entries expected %+v, got %+v", i, tt.wentries, st.snapshotEntries)
		}

		// make sure 'Append' did not manipulate the original entries
		if !reflect.DeepEqual(original, tt.entries) {
			t.Fatalf("#%d: original snapshot entries expected %+v, got %+v", i, tt.entries, original)
		}
	}
}

func Test_StorageStableInMemory_CreateSnapshot(t *testing.T) {
	var (
		ents = []raftpb.Entry{
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
		st := &StorageStableInMemory{snapshotEntries: ents}
		snap, err := st.CreateSnapshot(tt.snapshotIndex, &cs, data)
		if err != tt.werr {
			t.Fatalf("#%d: error expected %v, got %v", i, tt.werr, err)
		}
		if !reflect.DeepEqual(snap, tt.wsnap) {
			t.Fatalf("#%d: snap expected %+v, got %+v", i, tt.wsnap, snap)
		}
	}
}

func Test_StorageStableInMemory_Compact(t *testing.T) {
	ents := []raftpb.Entry{
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
		st := &StorageStableInMemory{snapshotEntries: ents}
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
