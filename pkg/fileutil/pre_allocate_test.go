package fileutil

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestPreallocate(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "preallocateTest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	f, err := ioutil.TempFile(p, "")
	if err != nil {
		t.Fatal(err)
	}

	size := int64(64 * 1000)
	if err = Preallocate(f, size, false); err != nil {
		t.Fatal(err)
	}

	stat, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}

	if stat.Size() != 0 { // no extend file, so file size stays the same
		t.Fatalf("expected 0, got %d", stat.Size())
	}
}

func TestPreallocateExtend(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "preallocateTest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	f, err := ioutil.TempFile(p, "")
	if err != nil {
		t.Fatal(err)
	}

	size := int64(64 * 1000)
	if err = Preallocate(f, size, true); err != nil {
		t.Fatal(err)
	}

	stat, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}

	if stat.Size() != size { // extend file, so file size changes
		t.Fatalf("expected %d, got %d", size, stat.Size())
	}
}
