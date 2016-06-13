package wal

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/gyuho/distdb/raftpb"
	"github.com/gyuho/distdb/walpb"
)

func TestOpenWAL(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	f, err := os.Create(filepath.Join(dir, getWALName(0, 0)))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	w, err := openWAL(dir, walpb.Snapshot{}, true)
	if err != nil {
		t.Fatal(err)
	}

	w.Lock()
	if name := filepath.Base(w.UnsafeLastFile().Name()); name != getWALName(0, 0) {
		t.Fatalf("expected %v, got %v", getWALName(0, 0), name)
	}
	if w.UnsafeLastFileSeq() != 0 {
		t.Fatalf("expected 0, got %d", w.UnsafeLastFileSeq())
	}
	w.Unlock()
	w.Close()

	emptyDir, err := ioutil.TempDir(os.TempDir(), "waltest_empty")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(emptyDir)

	if _, err = openWAL(emptyDir, walpb.Snapshot{}, true); err != ErrFileNotFound {
		t.Fatalf("expected %v, got %v", ErrFileNotFound, err)
	}
}

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
