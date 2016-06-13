package wal

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/gyuho/distdb/raftpb"
)

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
