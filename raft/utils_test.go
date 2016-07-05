package raft

import (
	"math"
	"reflect"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

func Test_util_limitEntries(t *testing.T) {
	entries := []raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 6}}
	tests := []struct {
		limitSize uint64
		wEntries  []raftpb.Entry
	}{
		{ // no limit
			math.MaxUint64,
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 6}},
		},

		{ // limitEntries must return at least one entry
			0,
			[]raftpb.Entry{{Index: 4, Term: 4}},
		},

		{ // limit to 2
			uint64(entries[0].Size() + entries[1].Size()),
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},

		{ // limit to 2
			uint64(entries[0].Size() + entries[1].Size() + entries[2].Size()/2),
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},

		{ // limit to 2
			uint64(entries[0].Size() + entries[1].Size() + entries[2].Size() - 1),
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},

		{ // limit to 3
			uint64(entries[0].Size() + entries[1].Size() + entries[2].Size()),
			[]raftpb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 6}},
		},
	}

	for i, tt := range tests {
		ents := limitEntries(tt.limitSize, entries...)
		if !reflect.DeepEqual(ents, tt.wEntries) {
			t.Fatalf("#%d: limit entries result expected %+v, got %+v", i, tt.wEntries, ents)
		}
	}
}
