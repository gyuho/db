package wal

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/wal/walpb"
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

func TestReleaseLocks(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// create a WAL
	w, err := Create(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// create 10 separate files
	for i := 0; i < 10; i++ {
		es := []raftpb.Entry{
			{Index: uint64(i)},
		}
		if err = w.Save(raftpb.HardState{}, es); err != nil {
			t.Fatal(err)
		}
		if err = w.UnsafeCutCurrent(); err != nil {
			t.Fatal(err)
		}
	}

	// release the locks at 5
	unlockIndex := uint64(5)
	w.ReleaseLocks(unlockIndex)

	// expected remaining are 4,5,6,7,8,9,10
	if len(w.lockedFiles) != 7 {
		t.Fatalf("len(w.lockedFiles) expected %d, got %d", 7, len(w.lockedFiles))
	}

	for i := range w.lockedFiles {
		var lockedIndex uint64
		_, lockedIndex, err = parseWALName(filepath.Base(w.lockedFiles[i].Name()))
		if err != nil {
			t.Fatal(err)
		}

		if lockedIndex != uint64(i+4) {
			t.Fatalf("#%d: lockedIndex expected %d, got %d", i, i+4, lockedIndex)
		}
	}

	// release all the locks except last
	w.ReleaseLocks(15)

	if len(w.lockedFiles) != 1 {
		t.Fatalf("len(w.lockedFiles) expected %d, got %d", 1, len(w.lockedFiles))
	}
	_, lockedIndex, err := parseWALName(filepath.Base(w.lockedFiles[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if lockedIndex != uint64(10) {
		t.Fatalf("lockedIndex expected %d, got %d", 10, lockedIndex)
	}
}
