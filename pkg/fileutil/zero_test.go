package fileutil

import (
	"io"
	"io/ioutil"
	"os"
	"testing"
)

// (etcd pkg.fileutil.TestZeroToEnd)
func Test_ZeroToEnd(t *testing.T) {
	f, err := ioutil.TempFile(os.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	b := make([]byte, 1024)
	for i := range b {
		b[i] = 12
	}
	if _, err = f.Write(b); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Seek(512, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if err = ZeroToEnd(f); err != nil {
		t.Fatal(err)
	}
	off, serr := f.Seek(0, io.SeekCurrent)
	if serr != nil {
		t.Fatal(serr)
	}
	if off != 512 {
		t.Fatalf("expected offset 512, got %d", off)
	}

	// reading the next 512 bytes from io.SeekCurrent
	b = make([]byte, 512)
	if _, err = f.Read(b); err != nil {
		t.Fatal(err)
	}
	for i := range b {
		if b[i] != 0 {
			t.Errorf("expected b[%d] = 0, got %d", i, b[i])
		}
	}
}
