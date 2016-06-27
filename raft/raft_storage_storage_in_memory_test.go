package raft

import (
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
