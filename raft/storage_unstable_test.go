package raft

import (
	"reflect"
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
	tests := []struct {
		incomingSnapshot *raftpb.Snapshot
		indexOffset      uint64
		entries          []raftpb.Entry

		index, term uint64

		windexOffset uint64
		wlen         int
	}{
		{
			nil, 0, []raftpb.Entry{},

			5, 1,

			0,
			0,
		},

		{
			nil, 5, []raftpb.Entry{{Index: 5, Term: 1}},

			5, 1, // persisting to the first entry of index 5

			6,
			0,
		},

		{
			nil, 5, []raftpb.Entry{{Index: 5, Term: 1}, {Index: 6, Term: 1}},

			5, 1, // persisting to the first entry of index 5

			6,
			1, // {Index: 6, Term: 1}
		},

		{
			nil, 6, []raftpb.Entry{{Index: 6, Term: 2}},

			6, 1, // persisting to the first entry of index 5, term mismatch

			6, // term mismatch, so the indexOffset did not increase
			1,
		},

		{
			nil, 5, []raftpb.Entry{{Index: 5, Term: 1}},

			4, 1, // persisting the old index

			5, // index mismatch, so nothing happens
			1,
		},

		{
			nil, 5, []raftpb.Entry{{Index: 5, Term: 1}},

			4, 2, // persisting the old index, but with different term

			5, // index mismatch, so nothing happens
			1,
		},

		{ // with snapshot
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 1}},
			5,
			[]raftpb.Entry{{Index: 5, Term: 1}},

			5, 1, // persisting to first entry

			6, // index match, so increases the index offset
			0,
		},

		{
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 1}},
			5,
			[]raftpb.Entry{{Index: 5, Term: 1}, {Index: 6, Term: 1}},

			5, 1, // persisting to first entry

			6, // index match, so increases the index offset
			1, // {Index: 6, Term: 1}
		},

		{
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 5, Term: 1}},
			6,
			[]raftpb.Entry{{Index: 6, Term: 2}},

			6, 1, // persisting to first entry, but mismatching term

			6, // mismatching term, so nothing happens
			1,
		},

		{
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 1}},
			5,
			[]raftpb.Entry{{Index: 5, Term: 1}},

			4, 1, // persisting to snapshot

			5,
			1,
		},

		{
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 2}},
			5,
			[]raftpb.Entry{{Index: 5, Term: 2}},

			4, 1, // persisting to old entry

			5, // old entry, so nothing happens
			1,
		},

		{
			&raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 2}},
			5,
			[]raftpb.Entry{{Index: 5, Term: 2}},

			4, 2, // persisting to old entry

			5, // old entry, so nothing happens
			1,
		},
	}

	for i, tt := range tests {
		su := storageUnstable{
			incomingSnapshot: tt.incomingSnapshot,
			indexOffset:      tt.indexOffset,
			entries:          tt.entries,
		}

		su.persistedEntriesAt(tt.index, tt.term)

		if su.indexOffset != tt.windexOffset {
			t.Fatalf("#%d: index offset = %d, want %d", i, su.indexOffset, tt.windexOffset)
		}

		if len(su.entries) != tt.wlen {
			t.Fatalf("#%d: len(su.entries) = %d, want %d", i, len(su.entries), tt.wlen)
		}
	}
}

func Test_storageUnstable_restoreIncomingSnapshot(t *testing.T) {
	su := storageUnstable{
		incomingSnapshot: &raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 4, Term: 1}},
		indexOffset:      5,
		entries:          []raftpb.Entry{{Index: 5, Term: 1}},
	}
	incomingSnapshot := raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 10, Term: 3}}

	su.restoreIncomingSnapshot(incomingSnapshot)

	if su.indexOffset != incomingSnapshot.Metadata.Index+1 {
		t.Fatalf("index offset expected %d, got %d", incomingSnapshot.Metadata.Index+1, su.indexOffset)
	}

	if len(su.entries) != 0 { // must be nil
		t.Fatalf("len(su.entries) expected 0, got %d", len(su.entries))
	}

	if !reflect.DeepEqual(su.incomingSnapshot, &incomingSnapshot) {
		t.Fatalf("incomingSnapshot expected %+v, got %+v", incomingSnapshot, su.incomingSnapshot)
	}
}

func Test_storageUnstable_truncateAndAppend(t *testing.T) {
	tests := []struct {
		incomingSnapshot *raftpb.Snapshot
		indexOffset      uint64
		entries          []raftpb.Entry

		entriesToAppend []raftpb.Entry

		windexOffset uint64
		wentries     []raftpb.Entry
	}{
		{ // direct append
			nil, 5, []raftpb.Entry{{Index: 5, Term: 1}},

			[]raftpb.Entry{{Index: 6, Term: 1}, {Index: 7, Term: 1}},

			5, []raftpb.Entry{{Index: 5, Term: 1}, {Index: 6, Term: 1}, {Index: 7, Term: 1}},
		},

		{ // replacing unstable entries
			nil, 5, []raftpb.Entry{{Index: 5, Term: 1}},

			[]raftpb.Entry{{Index: 5, Term: 2}, {Index: 6, Term: 2}},

			5, []raftpb.Entry{{Index: 5, Term: 2}, {Index: 6, Term: 2}},
		},

		{ // replacing unstable entries
			nil, 5, []raftpb.Entry{{Index: 5, Term: 1}},

			[]raftpb.Entry{{Index: 4, Term: 1}, {Index: 5, Term: 1}, {Index: 6, Term: 1}},

			4, []raftpb.Entry{{Index: 4, Term: 1}, {Index: 5, Term: 1}, {Index: 6, Term: 1}},
		},

		{ // replacing unstable entries
			nil, 5, []raftpb.Entry{{Index: 5, Term: 1}},

			[]raftpb.Entry{{Index: 4, Term: 2}, {Index: 5, Term: 2}, {Index: 6, Term: 2}},

			4, []raftpb.Entry{{Index: 4, Term: 2}, {Index: 5, Term: 2}, {Index: 6, Term: 2}},
		},

		{ // truncating existing entries
			nil, 5, []raftpb.Entry{{Index: 5, Term: 1}, {Index: 6, Term: 1}, {Index: 7, Term: 1}},

			[]raftpb.Entry{{Index: 6, Term: 2}},

			5, []raftpb.Entry{{Index: 5, Term: 1}, {Index: 6, Term: 2}},
		},

		{ // truncating existing entries
			nil, 5, []raftpb.Entry{{Index: 5, Term: 1}, {Index: 6, Term: 1}, {Index: 7, Term: 1}},

			[]raftpb.Entry{{Index: 7, Term: 2}, {Index: 8, Term: 2}},

			5, []raftpb.Entry{{Index: 5, Term: 1}, {Index: 6, Term: 1}, {Index: 7, Term: 2}, {Index: 8, Term: 2}},
		},
	}

	for i, tt := range tests {
		su := storageUnstable{
			incomingSnapshot: tt.incomingSnapshot,
			indexOffset:      tt.indexOffset,
			entries:          tt.entries,
		}

		su.truncateAndAppend(tt.entriesToAppend)

		if su.indexOffset != tt.windexOffset {
			t.Fatalf("#%d: index offset = %d, want %d", i, su.indexOffset, tt.windexOffset)
		}

		if !reflect.DeepEqual(su.entries, tt.wentries) {
			t.Fatalf("#%d: su.entries expected %+v, got %+v", i, su.entries, tt.wentries)
		}
	}
}
