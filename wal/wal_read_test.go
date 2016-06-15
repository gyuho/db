package wal

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/gyuho/distdb/raftpb"
	"github.com/gyuho/distdb/walpb"
)

func TestOpenWALRead(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// create WAL
	w, err := Create(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// make 10 separate files
	for i := 0; i < 10; i++ {
		entries := []raftpb.Entry{
			{Index: uint64(i)},
		}
		if err = w.Save(raftpb.HardState{}, entries); err != nil {
			t.Fatal(err)
		}
		if err = w.UnsafeCutCurrent(); err != nil {
			t.Fatal(err)
		}
	}

	// release the lock at 5
	w.ReleaseLocks(uint64(5))

	// all are still readable despite flocks
	w2, err := OpenWALRead(dir, walpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()

	_, _, entries, err := w2.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if idx := entries[len(entries)-1].Index; idx != 9 {
		t.Fatalf("expected 9, got %d", idx)
	}
}

func TestReadAllEmpty(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	w, err := Create(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err = w.UnsafeEncodeSnapshotAndFdatasync(&walpb.Snapshot{}); err != nil {
		t.Fatal(err)
	}
	w.Close()

	w, err = OpenWALRead(dir, walpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if _, _, _, err = w.ReadAll(); err != nil {
		t.Fatal(err)
	}
}
