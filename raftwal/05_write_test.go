package raftwal

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gyuho/db/pkg/fileutil"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftwal/raftwalpb"
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

	w, err := openWAL(dir, raftwalpb.Snapshot{}, true)
	if err != nil {
		t.Fatal(err)
	}

	w.Lock()
	if name := filepath.Base(w.unsafeLastFile().Name()); name != getWALName(0, 0) {
		t.Fatalf("expected %v, got %v", getWALName(0, 0), name)
	}
	if w.unsafeLastFileSeq() != 0 {
		t.Fatalf("expected 0, got %d", w.unsafeLastFileSeq())
	}
	w.Unlock()
	w.Close()

	emptyDir, err := ioutil.TempDir(os.TempDir(), "waltest_empty")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(emptyDir)

	if _, err = openWAL(emptyDir, raftwalpb.Snapshot{}, true); err != ErrFileNotFound {
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
		if err = w.unsafeCutCurrent(); err != nil {
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

// createEmptyEntries creates empty slice of entries for testing.
func createEmptyEntries(num int) [][]raftpb.Entry {
	entries := make([][]raftpb.Entry, num)
	for i := 0; i < num; i++ {
		entries[i] = []raftpb.Entry{
			{Index: uint64(i + 1)},
		}
	}
	return entries
}

func Test_unsafeEncodeHardState(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	w := &WAL{
		enc: newEncoder(buf, 0),
	}
	if err := w.unsafeEncodeHardState(&raftpb.HardState{}); err != nil {
		t.Fatal(err)
	}
	if len(buf.Bytes()) != 0 {
		t.Fatalf("len(buf.Bytes) expected 0, got %d", len(buf.Bytes()))
	}
}

// (etcd pkg.wal.TestNew)
func TestCreate(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	data := []byte("metadata")

	w, err := Create(dir, data)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	fpath := filepath.Base(w.unsafeLastFile().Name())
	if fpath != getWALName(0, 0) {
		t.Fatalf("expected %q, got %q", getWALName(0, 0), fpath)
	}

	// WAL is created with preallocation with segment size
	offset, err := w.unsafeLastFile().Seek(0, os.SEEK_CUR)
	if err != nil {
		t.Fatal(err)
	}
	if offset != minSectorSize/byteBitN { // 64
		t.Fatalf("offset expected %d, got %d", minSectorSize/byteBitN, offset)
	}

	// read bytes from the WAL file
	f, err := os.Open(filepath.Join(dir, fpath))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	offsetBuf := make([]byte, offset)
	if _, err = io.ReadFull(f, offsetBuf); err != nil {
		t.Fatal(err)
	}

	// encode empty record to compare with the bytes
	emptyBuf := new(bytes.Buffer)
	enc := newEncoder(emptyBuf, 0)

	// 1. encode CRC
	if err = enc.encode(&raftwalpb.Record{
		Type: raftwalpb.RECORD_TYPE_CRC,
		CRC:  0,
	}); err != nil {
		t.Fatal(err)
	}

	// 2. encode metadata
	if err = enc.encode(&raftwalpb.Record{
		Type: raftwalpb.RECORD_TYPE_METADATA,
		Data: data,
	}); err != nil {
		t.Fatal(err)
	}

	// 3. encode empty snapshot
	snap := &raftwalpb.Snapshot{}
	snapData, err := snap.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if err = enc.encode(&raftwalpb.Record{
		Type: raftwalpb.RECORD_TYPE_SNAPSHOT,
		Data: snapData,
	}); err != nil {
		t.Fatal(err)
	}
	enc.flush()

	if !bytes.Equal(offsetBuf, emptyBuf.Bytes()) {
		t.Fatalf("expected %q, got %q", offsetBuf, emptyBuf.Bytes())
	}
}

// (etcd pkg.wal.TestNewForInitedDir)
func TestCreateErrExist(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if _, err = os.Create(filepath.Join(dir, getWALName(0, 0))); err != nil {
		t.Fatal(err)
	}

	if _, err = Create(dir, nil); err == nil || err != os.ErrExist {
		t.Fatalf("unexpected error %v", err)
	}
}

// (etcd pkg.wal.TestRestartCreateWal)
func TestCreateInterrupted(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// create temporary directory to make it look like initialization got interrupted
	tmpDir := filepath.Clean(dir) + ".tmp"
	if err = fileutil.MkdirAll(tmpDir); err != nil {
		t.Fatal(err)
	}
	if _, err = os.OpenFile(filepath.Join(tmpDir, "test"), os.O_WRONLY|os.O_CREATE, fileutil.PrivateFileMode); err != nil {
		t.Fatal(err)
	}

	var (
		metadata = []byte("metadata")
		w        *WAL
	)
	w, err = Create(dir, metadata)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	if fileutil.DirHasFiles(tmpDir) {
		t.Fatalf("%q should have been renamed (should not exist)", tmpDir)
	}

	w, err = OpenWALRead(dir, raftwalpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	metadata2, _, _, err := w.ReadAll()
	if err != nil || !bytes.Equal(metadata2, metadata) {
		t.Fatalf("expected <nil, %q>, got <%v, %q>", metadata, nil, metadata2)
	}
}

// (etcd pkg.wal.TestRecover)
func TestSave(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	metadata := []byte("metadata")
	w, err := Create(dir, metadata)
	if err != nil {
		t.Fatal(err)
	}

	// save snapshot
	if err = w.UnsafeEncodeSnapshotAndFdatasync(&raftwalpb.Snapshot{}); err != nil {
		t.Fatal(err)
	}

	// save some entries
	entries := []raftpb.Entry{
		{Index: 1, Term: 1, Data: []byte{1}},
		{Index: 2, Term: 2, Data: []byte{2}},
	}
	if err = w.Save(raftpb.HardState{}, entries); err != nil {
		t.Fatal(err)
	}

	// save hard states
	hardstates := []raftpb.HardState{
		{VotedFor: 1, CommittedIndex: 1, Term: 1},
		{VotedFor: 2, CommittedIndex: 2, Term: 2},
	}
	for _, st := range hardstates {
		if err = w.Save(st, nil); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	w, err = OpenWALRead(dir, raftwalpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	metadata2, hardstate, entries2, err := w.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(metadata, metadata2) {
		t.Fatalf("expected %q, got %q", metadata, metadata2)
	}

	if !reflect.DeepEqual(entries, entries2) {
		t.Fatalf("expected %+v, got %+v", entries, entries2)
	}

	// only the last hardstate is written (overwritten)
	st := hardstates[len(hardstates)-1]
	if !reflect.DeepEqual(st, hardstate) {
		t.Fatalf("expected %+v, got %+v", st, hardstate)
	}
}

// (etcd pkg.wal.TestCut)
func Test_unsafeCutCurrent(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	w, err := Create(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	hardstate := raftpb.HardState{Term: 1}
	if err = w.Save(hardstate, nil); err != nil {
		t.Fatal(err)
	}

	if err = w.unsafeCutCurrent(); err != nil {
		t.Fatal(err)
	}

	if fpath := filepath.Base(w.unsafeLastFile().Name()); fpath != getWALName(1, 1) {
		t.Fatalf("name expected %q, got %q", getWALName(1, 1), fpath)
	}

	entries := []raftpb.Entry{{Term: 1, Index: 1, Data: []byte{1}}}
	if err = w.Save(raftpb.HardState{}, entries); err != nil {
		t.Fatal(err)
	}

	if err = w.unsafeCutCurrent(); err != nil {
		t.Fatal(err)
	}

	snapshot := raftwalpb.Snapshot{Term: 1, Index: 2}
	if err = w.UnsafeEncodeSnapshotAndFdatasync(&snapshot); err != nil {
		t.Fatal(err)
	} // this does fsync

	if fpath := filepath.Base(w.unsafeLastFile().Name()); fpath != getWALName(2, 2) {
		t.Fatalf("expected %q, got %q", getWALName(2, 2), fpath)
	}

	f, err := os.Open(filepath.Join(dir, getWALName(2, 2)))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w2 := &WAL{
		dec:               newDecoder(f),
		readStartSnapshot: snapshot,
	}
	_, hardstate2, _, err := w2.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(hardstate, hardstate2) {
		t.Fatalf("expected %+v, got %+v", hardstate, hardstate2)
	}
}

// (etcd pkg.wal.TestRecoverAfterCut)
func Test_unsafeCutCurrent_Recover(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	metadata := []byte("metadata")
	w, err := Create(dir, metadata)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		if err = w.UnsafeEncodeSnapshotAndFdatasync(&raftwalpb.Snapshot{Index: uint64(i)}); err != nil {
			t.Fatal(err)
		}

		entries := []raftpb.Entry{{Index: uint64(i)}}
		if err = w.Save(raftpb.HardState{}, entries); err != nil {
			t.Fatal(err)
		}
		if err = w.unsafeCutCurrent(); err != nil {
			t.Fatal(err)
		}
	}

	w.Close()

	if err = os.Remove(filepath.Join(dir, getWALName(4, 4))); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		wr, err := OpenWALRead(dir, raftwalpb.Snapshot{Index: uint64(i)})
		if err != nil {
			if i <= 4 {
				if err != ErrFileNotFound {
					t.Fatalf("#%d: expected %v, got %v", i, ErrFileNotFound, err)
				}
			} else {
				t.Fatalf("#%d: error %v", i, err)
			}
			continue
		}

		metadata2, _, entries, err := wr.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(metadata, metadata2) {
			t.Fatalf("#%d: expected %q, got %q", i, metadata, metadata2)
		}

		for j, ent := range entries {
			if ent.Index != uint64(i+j+1) {
				t.Fatalf("#%d.%d: expected %d, got %d", i, j, i+j+1, ent.Index)
			}
		}

		wr.Close()
	}
}

// (etcd pkg.wal.TestTailWriteNoSlackSpace)
func TestTailWritesUnused(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// create WAL
	w, err := Create(dir, []byte("metadata"))
	if err != nil {
		t.Fatal(err)
	}

	// write some entries
	for i := 1; i <= 5; i++ {
		entries := []raftpb.Entry{
			{Term: 1, Index: uint64(i), Data: []byte{byte(i)}},
		}
		if err = w.Save(raftpb.HardState{Term: 1}, entries); err != nil {
			t.Fatal(err)
		}
	}

	// remove the unused space (slack space) by truncating
	offset, err := w.unsafeLastFile().Seek(0, os.SEEK_CUR)
	if err != nil {
		t.Fatal(err)
	}
	if err = w.unsafeLastFile().Truncate(offset); err != nil {
		t.Fatal(err)
	}
	w.Close()

	w, err = OpenWALWrite(dir, raftwalpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, entries, err := w.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("len(entries) expected 5, got %d", len(entries))
	}

	// write more data
	for i := 6; i <= 10; i++ {
		ets := []raftpb.Entry{
			{Term: 1, Index: uint64(i), Data: []byte{byte(i)}},
		}
		if err = w.Save(raftpb.HardState{Term: 1}, ets); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	// verify the writes
	w, err = OpenWALRead(dir, raftwalpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, entries, err = w.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 10 {
		t.Fatalf("len(entries) expected 10, got %d", len(entries))
	}
}
