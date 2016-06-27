package raft

import (
	"math"
	"reflect"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

func Test_StorageInMemory_Term(t *testing.T) {
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
		st := &StorageInMemory{snapshotEntries: ents}
		term, err := st.Term(tt.entryIdx)
		if err != tt.werr {
			t.Fatalf("#%d: error expected %v, got %v", i, tt.werr, err)
		}
		if term != tt.wterm {
			t.Fatalf("#%d: term expected %d, got %d", i, tt.wterm, term)
		}
	}
}

func Test_StorageInMemory_Entries(t *testing.T) {
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
			2,
			6,
			math.MaxUint64,
			ErrCompacted,
			nil,
		},

		{
			3,
			4,
			math.MaxUint64,
			ErrCompacted,
			nil,
		},

		{
			4,
			5,
			math.MaxUint64,
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}},
		},

		{
			4,
			6,
			math.MaxUint64,
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},

		{
			4,
			7,
			math.MaxUint64,
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 6}},
		},

		// even limitSize 0 should return at least one entry
		{
			4,
			7,
			0,
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}},
		},

		// limit to 2 entries
		{
			4,
			7,
			uint64(ents[1].Size() + ents[2].Size()),
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},

		// limit to 2 entries
		{
			4,
			7,
			uint64(ents[1].Size() + ents[2].Size() + ents[3].Size()/2),
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
		}, // limit to 2 entries

		{
			4,
			7,
			uint64(ents[1].Size() + ents[2].Size() + ents[3].Size() - 1),
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},

		{
			4,
			7,
			uint64(ents[1].Size() + ents[2].Size() + ents[3].Size()),
			nil,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 6}},
		},
	}

	for i, tt := range tests {
		st := &StorageInMemory{snapshotEntries: ents}
		entries, err := st.Entries(tt.startIdx, tt.endIdx, tt.limitSize)
		if err != tt.werr {
			t.Fatalf("#%d: error expected %v, got %v", i, tt.werr, err)
		}
		if !reflect.DeepEqual(entries, tt.wentries) {
			t.Fatalf("#%d: entries expected %+v, got %+v", i, tt.wentries, entries)
		}
	}
}
