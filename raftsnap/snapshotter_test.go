package raftsnap

import (
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

var testSnap = &raftpb.Snapshot{
	Data: []byte("testdata"),
	Metadata: raftpb.SnapshotMetadata{
		ConfigState: raftpb.ConfigState{
			IDs: []uint64{1, 2, 3},
		},
		Index: 1,
		Term:  1,
	},
}

// (etcd snap.TestSaveAndLoad)
func Test_Snapshotter_SaveAndLoad(t *testing.T) {
	dir := path.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	ss := New(dir)
	err = ss.save(testSnap)
	if err != nil {
		t.Fatal(err)
	}

	g, err := ss.Load()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !reflect.DeepEqual(g, testSnap) {
		t.Fatalf("snap = %#v, want %#v", g, testSnap)
	}
}

// (etcd snap.TestBadCRC)
func Test_Snapshotter_BadCRC(t *testing.T) {
	dir := path.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	ss := New(dir)
	err = ss.save(testSnap)
	if err != nil {
		t.Fatal(err)
	}
	oldTable := crcTable
	defer func() {
		crcTable = oldTable
	}()

	// switch to use another crc table
	// fake a crc mismatch
	crcTable = crc32.MakeTable(crc32.Koopman)

	_, err = Read(filepath.Join(dir, fmt.Sprintf("%016x-%016x.snap", 1, 1)))
	if err == nil || err != ErrCRCMismatch {
		t.Fatalf("err = %v, want %v", err, ErrCRCMismatch)
	}
}

// (etcd snap.TestFailback)
func Test_Snapshotter_Failback(t *testing.T) {
	dir := path.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	large := fmt.Sprintf("%016x-%016x-%016x.snap", 0xFFFF, 0xFFFF, 0xFFFF)
	err = ioutil.WriteFile(filepath.Join(dir, large), []byte("bad data"), 0666)
	if err != nil {
		t.Fatal(err)
	}

	ss := New(dir)
	err = ss.save(testSnap)
	if err != nil {
		t.Fatal(err)
	}

	g, err := ss.Load()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !reflect.DeepEqual(g, testSnap) {
		t.Fatalf("snap = %#v, want %#v", g, testSnap)
	}
	if f, err := os.Open(filepath.Join(dir, large) + ".broken"); err != nil {
		t.Fatal("broken snapshot does not exist")
	} else {
		f.Close()
	}
}

// (etcd snap.TestLoadNewestSnap)
func Test_Snapshotter_LoadNewestSnap(t *testing.T) {
	dir := path.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	ss := New(dir)
	err = ss.save(testSnap)
	if err != nil {
		t.Fatal(err)
	}

	newSnap := *testSnap
	newSnap.Metadata.Index = 5
	err = ss.save(&newSnap)
	if err != nil {
		t.Fatal(err)
	}

	g, err := ss.Load()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !reflect.DeepEqual(g, &newSnap) {
		t.Fatalf("snap = %#v, want %#v", g, &newSnap)
	}
}

// (etcd snap.TestNoSnapshot)
func Test_Snapshotter_NoSnapshot(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	ss := New(dir)
	_, err = ss.Load()
	if err != ErrNoSnapshot {
		t.Fatalf("err = %v, want %v", err, ErrNoSnapshot)
	}
}

// (etcd snap.TestEmptySnapshot)
func Test_Snapshotter_EmptySnapshot(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	err = ioutil.WriteFile(filepath.Join(dir, "1.snap"), []byte(""), 0x700)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Read(filepath.Join(dir, "1.snap"))
	if err != ErrEmptySnapshot {
		t.Fatalf("err = %v, want %v", err, ErrEmptySnapshot)
	}
}

// (etcd snap.TestAllSnapshotBroken)
func Test_Snapshotter_AllSnapshotBroken(t *testing.T) {
	dir := path.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	err = ioutil.WriteFile(filepath.Join(dir, "1.snap"), []byte("bad"), 0x700)
	if err != nil {
		t.Fatal(err)
	}

	ss := New(dir)
	_, err = ss.Load()
	if err != ErrNoSnapshot {
		t.Fatalf("err = %v, want %v", err, ErrNoSnapshot)
	}
}
