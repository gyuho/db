package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gyuho/db/pkg/fileutil"
)

func Test_dataStore(t *testing.T) {
	ds := newDataStore(make(chan []byte), make(chan []byte))
	defer ds.stop()

	donec := make(chan struct{})
	go func() {
		defer close(donec)

		bts := <-ds.propc
		// assume this is agreed by consensus

		ds.commitc <- bts
	}()
	ds.propose(context.TODO(), keyValue{"foo", "bar"})
	<-donec

	time.Sleep(10 * time.Millisecond)

	val, ok := ds.get("foo")
	if !ok {
		t.Fatal("ok expected true, got false")
	}
	if val != "bar" {
		t.Fatalf("value expected %q, got %q", "bar", val)
	}

	close(ds.errc)
	if err := <-ds.errc; err != nil {
		t.Fatal(err)
	}
}

func Test_dataStore_createSnapshot(t *testing.T) {
	tm := map[string]string{"foo": "bar"}
	fpath := filepath.Join(os.TempDir(), "testsnapshot")
	os.RemoveAll(fpath)

	ds := newDataStore(make(chan []byte), make(chan []byte))
	defer ds.stop()

	ds.store = tm

	bts, err := ds.createSnapshot()
	if err != nil {
		t.Fatal(err)
	}

	if err := fileutil.WriteSync(fpath, bts, fileutil.PrivateFileMode); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fpath)

	ds.store = nil

	f, err := fileutil.OpenToRead(fpath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := ds.loadSnapshot(f); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(ds.store, tm) {
		t.Fatalf("store expected %+v, got %+v", tm, ds.store)
	}

	close(ds.errc)
	if err := <-ds.errc; err != nil {
		t.Fatal(err)
	}
}
