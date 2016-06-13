package wal

import (
	"fmt"
	"testing"
)

func TestWALName(t *testing.T) {
	name := "00000000000000b2-000000000456a937.wal"
	wname := getWALName(178, 72788279)
	if name != wname {
		t.Fatalf("name expected %q, got %q", name, wname)
	}
	seq, index, err := parseWALName(name)
	if err != nil {
		t.Fatal(err)
	}
	if "b2" != fmt.Sprintf("%x", seq) {
		t.Fatalf("seq expected 'b2', got %x", seq)
	}
	if "456a937" != fmt.Sprintf("%x", index) {
		t.Fatalf("index expected '456a937', got %x", index)
	}
}

func TestParseWALName(t *testing.T) {
	tests := []struct {
		name string
		wseq uint64
		wcur uint64
		ok   bool
	}{
		{
			"0000000000000000-0000000000000000.wal",
			0,
			0,
			true,
		},
		{
			"0000000000000000.wal",
			0,
			0,
			false, // wrong length
		},
		{
			"0000000000000000-0000000000000000.snap",
			0,
			0,
			false, // wrong extension
		},
	}

	for i, tt := range tests {
		seq, curIndex, err := parseWALName(tt.name)
		if ok := err == nil; ok != tt.ok {
			t.Fatalf("#%d: ok expected %v, got %v", i, tt.ok, ok)
		}

		if tt.wseq != seq {
			t.Fatalf("#%d: seq expected %x, got %x", i, tt.wseq, seq)
		}

		if tt.wcur != curIndex {
			t.Fatalf("#%d: curIndex expected %x, got %x", i, tt.wcur, curIndex)
		}
	}
}

func TestSearchLastWALIndex(t *testing.T) {
	tests := []struct {
		names   []string
		findIdx uint64
		wIdx    int // last index equal or smaller
	}{
		{
			[]string{
				"0000000000000000-0000000000000000.wal",
				"0000000000000001-0000000000001000.wal",
				"0000000000000002-0000000000002000.wal",
			},
			0x1000,
			1,
		},
		{
			[]string{
				"0000000000000001-0000000000004000.wal",
				"0000000000000002-0000000000003000.wal",
				"0000000000000003-0000000000005000.wal",
			},
			0x4000,
			1,
		},
		{
			[]string{
				"0000000000000001-0000000000002000.wal",
				"0000000000000002-0000000000003000.wal",
				"0000000000000003-0000000000005000.wal",
			},
			0x1000,
			-1,
		},
	}

	for i, tt := range tests {
		idx := searchLastWALIndex(tt.names, tt.findIdx)
		if idx != tt.wIdx {
			t.Fatalf("#%d: idx expected %d, got %d", i, tt.wIdx, idx)
		}
	}
}
