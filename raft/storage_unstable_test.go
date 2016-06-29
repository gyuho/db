package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

func Test_storageUnstable_maybeFirstIndex(t *testing.T) {
	tests := []struct {
		incomingSnapshot *raftpb.Snapshot
		indexOffset      uint64
		entries          []raftpb.Entry

		windex uint64
		wok    bool
	}{
		{
			nil,
			5,
			[]raftpb.Entry{{Index: 5, Term: 1}},

			0, false,
		},

		{ // empty unstable
			nil,
			0,
			[]raftpb.Entry{},

			0, false,
		},

		{
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 1}},
			5,
			[]raftpb.Entry{{Index: 5, Term: 1}},

			5, true,
		},

		{
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 1}},
			5,
			[]raftpb.Entry{},

			5, true,
		},
	}

	for i, tt := range tests {
		su := storageUnstable{
			incomingSnapshot: tt.incomingSnapshot,
			indexOffset:      tt.indexOffset,
			entries:          tt.entries,
		}

		index, ok := su.maybeFirstIndex()

		if index != tt.windex {
			t.Fatalf("#%d: index = %d, want %d", i, index, tt.windex)
		}

		if ok != tt.wok {
			t.Fatalf("#%d: ok = %t, want %t", i, ok, tt.wok)
		}
	}
}

func Test_storageUnstable_maybeLastIndex(t *testing.T) {
	tests := []struct {
		incomingSnapshot *raftpb.Snapshot
		indexOffset      uint64
		entries          []raftpb.Entry

		windex uint64
		wok    bool
	}{
		{ // last in entries
			nil,
			5,
			[]raftpb.Entry{{Index: 5, Term: 1}},

			5, true,
		},

		{ // empty unstable
			nil,
			0,
			[]raftpb.Entry{},

			0, false,
		},

		{ // last in entries
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 1}},
			5,
			[]raftpb.Entry{{Index: 5, Term: 1}},

			5, true,
		},

		{ // last in snapshot
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 1}},
			5,
			[]raftpb.Entry{},

			4, true,
		},
	}

	for i, tt := range tests {
		su := storageUnstable{
			incomingSnapshot: tt.incomingSnapshot,
			indexOffset:      tt.indexOffset,
			entries:          tt.entries,
		}

		index, ok := su.maybeLastIndex()

		if index != tt.windex {
			t.Fatalf("#%d: index = %d, want %d", i, index, tt.windex)
		}

		if ok != tt.wok {
			t.Fatalf("#%d: ok = %t, want %t", i, ok, tt.wok)
		}
	}
}

func Test_storageUnstable_maybeTerm(t *testing.T) {
	tests := []struct {
		incomingSnapshot *raftpb.Snapshot
		indexOffset      uint64
		entries          []raftpb.Entry

		index uint64

		wterm uint64
		wok   bool
	}{
		{
			nil,
			5,
			[]raftpb.Entry{{Index: 5, Term: 1}},

			5,

			1, true,
		},

		{
			nil,
			5,
			[]raftpb.Entry{{Index: 5, Term: 1}},

			6,

			0, false,
		},

		{
			nil,
			5,
			[]raftpb.Entry{{Index: 5, Term: 1}},

			4,

			0, false,
		},

		{ // empty unstable
			nil,
			0,
			[]raftpb.Entry{},

			5,

			0, false,
		},

		{
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 1}},
			5,
			[]raftpb.Entry{{Index: 5, Term: 1}},

			5,

			1, true,
		},

		{
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 1}},
			5,
			[]raftpb.Entry{{Index: 5, Term: 1}},

			6,

			0, false,
		},

		{ // term from snapshot
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 1}},
			5,
			[]raftpb.Entry{{Index: 5, Term: 1}},

			4,

			1, true,
		},

		{
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 1}},
			5,
			[]raftpb.Entry{},

			5,

			0, false,
		},

		{ // term from snapshot
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 1}},
			5,
			[]raftpb.Entry{},

			4,

			1, true,
		},
	}

	for i, tt := range tests {
		su := storageUnstable{
			incomingSnapshot: tt.incomingSnapshot,
			indexOffset:      tt.indexOffset,
			entries:          tt.entries,
		}

		term, ok := su.maybeTerm(tt.index)

		if term != tt.wterm {
			t.Fatalf("#%d: term = %d, want %d", i, term, tt.wterm)
		}

		if ok != tt.wok {
			t.Fatalf("#%d: ok = %t, want %t", i, ok, tt.wok)
		}
	}
}

func Test_storageUnstable_persistedEntriesAt(t *testing.T) {

}

func Test_storageUnstable_restoreIncomingSnapshot(t *testing.T) {

}

func Test_storageUnstable_truncateAndAppend(t *testing.T) {

}
