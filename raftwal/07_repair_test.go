package raftwal

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftwal/raftwalpb"
)

type corruptFunc func(string, int64) error

// (etcd wal.testRepair)
func testRepair(t *testing.T, expectedEntriesN int, entries [][]raftpb.Entry, corrupt corruptFunc) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// create a WAL
	w, err := Create(dir, nil) // empty metadata
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	for _, es := range entries {
		if err = w.Save(raftpb.HardState{}, es); err != nil {
			t.Fatal(err)
		}
	}

	// move current position to 0
	offset, err := w.unsafeLastFile().Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	// corrupt the data with the offset
	if err = corrupt(dir, offset); err != nil {
		t.Fatal(err)
	}

	// verify the data is now corrupted
	w, err = OpenWALWrite(dir, raftwalpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = w.ReadAll()
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("expected %v, got %v", io.ErrUnexpectedEOF, err)
	}
	w.Close()

	// repair the corrupted WAL file
	if ok := Repair(dir); !ok {
		t.Fatalf("Repair failed on %q", dir)
	}

	// read the repaired WAL
	w, err = OpenWALWrite(dir, raftwalpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, walEntries, err := w.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(walEntries) != expectedEntriesN {
		t.Fatalf("len(walEntries) expected %d, got %d", expectedEntriesN, len(walEntries))
	}

	// write more data to the repaired WAL
	for i := 0; i < 10; i++ {
		es := []raftpb.Entry{
			{Index: uint64(expectedEntriesN + i + 1)},
		}
		if err = w.Save(raftpb.HardState{}, es); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	// read WAL again to ensure all writes are successful
	w, err = OpenWALRead(dir, raftwalpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, walEntries, err = w.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(walEntries) != expectedEntriesN+10 {
		t.Fatalf("len(walEntries) expected %d, got %d", expectedEntriesN+10, len(walEntries))
	}
}

// (etcd wal.TestRepairTruncate)
func TestRepair_TruncatedWAL(t *testing.T) {
	entriesN := 10
	entries := createEmptyEntries(entriesN)

	expectedEntriesN := 9

	corruptByTruncate := func(dir string, offset int64) error {
		f, err := openLastWALFile(dir)
		if err != nil {
			return err
		}
		defer f.Close()

		// truncate the last Record by 5 bytes
		return f.Truncate(offset - 5)
	}

	testRepair(t, expectedEntriesN, entries, corruptByTruncate)
}

// (etcd wal.TestRepairWriteTearLast)
func TestRepair_TearLastRecord(t *testing.T) {
	entriesN := 50
	entries := createEmptyEntries(entriesN)

	expectedEntriesN := 40

	corruptTornLast := func(dir string, offset int64) error {
		f, err := openLastWALFile(dir)
		if err != nil {
			return err
		}
		defer f.Close()

		// 512-byte is minSectorSize, so it perfectly aligns the last record
		if offset < 1024 {
			return fmt.Errorf("expected offset > 1024, got %d", offset)
		}

		// When we truncate by 1024, it changes the size of the file.
		// Since 1024 > offset(original file size), it will populate 1024 with zero values.
		if err = f.Truncate(1024); err != nil {
			return err
		}
		// truncate back
		if err = f.Truncate(offset); err != nil {
			return err
		}

		return nil
	}

	testRepair(t, expectedEntriesN, entries, corruptTornLast)
}

// (etcd wal.TestRepairWriteTearMiddle)
func TestRepair_TearMiddleRecord(t *testing.T) {
	// 4096-byte, easy to corrupt middle sector
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i)
	}

	entriesN := 5
	entries := createEmptyEntries(entriesN)
	for i := range entries {
		entries[i][0].Data = data
	}

	expectedEntriesN := 1

	corruptTornLast := func(dir string, offset int64) error {
		f, err := openLastWALFile(dir)
		if err != nil {
			return err
		}
		defer f.Close()

		// corrupt the middle sector of the second record
		_, err = f.WriteAt(make([]byte, 512), 4096+512)

		return err
	}

	testRepair(t, expectedEntriesN, entries, corruptTornLast)
}
