package raftwal

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/gyuho/db/pkg/fileutil"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftwal/raftwalpb"
)

// (etcd wal.TestOpenForRead)
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
		if err = w.unsafeCutCurrent(); err != nil {
			t.Fatal(err)
		}
	}

	// release the lock at 5
	w.ReleaseLocks(uint64(5))

	// all are still readable despite flocks
	w2, err := OpenWALRead(dir, raftwalpb.Snapshot{})
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

// (etcd wal.TestOpenAtUncommittedIndex)
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

	if err = w.UnsafeEncodeSnapshotAndFdatasync(&raftwalpb.Snapshot{}); err != nil {
		t.Fatal(err)
	}
	if err = w.Save(raftpb.HardState{}, []raftpb.Entry{{Index: 0}}); err != nil {
		t.Fatal(err)
	}
	w.Close()

	w, err = OpenWALRead(dir, raftwalpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if _, _, _, err = w.ReadAll(); err != nil {
		t.Fatal(err)
	}
}

// (etcd wal.TestOpenOnTornWrite)
func Test_ReadAll_torn_write(t *testing.T) {
	maxEntries := 40
	clobberIdx := 20
	overwriteEntries := 5

	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// get offset of end of each saved entry
	offsets := make([]int64, maxEntries)
	for i := range offsets {
		if err = w.Save(raftpb.HardState{}, []raftpb.Entry{{Index: uint64(i)}}); err != nil {
			t.Fatal(err)
		}
		if offsets[i], err = w.unsafeLastFile().Seek(0, io.SeekCurrent); err != nil {
			t.Fatal(err)
		}
	}

	fn := filepath.Join(p, filepath.Base(w.unsafeLastFile().Name()))
	w.Close()

	// clobber some entry with 0's to simulate a torn write
	f, ferr := os.OpenFile(fn, os.O_WRONLY, fileutil.PrivateFileMode)
	if ferr != nil {
		t.Fatal(ferr)
	}
	defer f.Close()

	_, err = f.Seek(offsets[clobberIdx], io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}

	zeros := make([]byte, offsets[clobberIdx+1]-offsets[clobberIdx])
	_, err = f.Write(zeros)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	w, err = OpenWALWrite(p, raftwalpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}

	// seek up to clobbered entry
	_, _, _, err = w.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	// write a few entries past the clobbered entry
	for i := 0; i < overwriteEntries; i++ {
		// Index is different from old, truncated entries
		es := []raftpb.Entry{{Index: uint64(i + clobberIdx), Data: []byte("new")}}
		if err = w.Save(raftpb.HardState{}, es); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	// read back the entries, confirm number of entries matches expectation
	w, err = OpenWALRead(p, raftwalpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}

	_, _, ents, rerr := w.ReadAll()
	if rerr != nil {
		// CRC error? the old entries were likely never truncated away
		t.Fatal(rerr)
	}
	wEntries := (clobberIdx - 1) + overwriteEntries
	if len(ents) != wEntries {
		t.Fatalf("expected len(ents) = %d, got %d", wEntries, len(ents))
	}
}
