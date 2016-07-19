package fileutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// (etcd pkg.fileutil.TestPurgeFile)
func TestPurgeFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "purgefile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	for i := 0; i < 3; i++ {
		var f *os.File
		f, err = os.Create(filepath.Join(dir, fmt.Sprintf("%d.test", i)))
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}

	stopc := make(chan struct{})
	purgec := make(chan string, 10)

	// keep at most 3 most recent files
	errc := purgeFile(dir, "test", 3, time.Millisecond, stopc, purgec)
	select {
	case f := <-purgec:
		t.Fatalf("unexpected purge on %q", f)
	case <-time.After(10 * time.Millisecond):
	}

	// create 6 more files
	for i := 4; i < 10; i++ {
		go func(n int) {
			var f *os.File
			f, err = os.Create(filepath.Join(dir, fmt.Sprintf("%d.test", n)))
			if err != nil {
				t.Fatal(err)
			}
			f.Close()
		}(i)
	}

	// watch files purge away
	for i := 4; i < 10; i++ {
		select {
		case <-purgec:
		case <-time.After(time.Second):
			t.Fatal("purge took too long")
		}
	}

	fnames, rerr := ReadDir(dir)
	if rerr != nil {
		t.Fatal(rerr)
	}
	wnames := []string{"7.test", "8.test", "9.test"}
	if !reflect.DeepEqual(fnames, wnames) {
		t.Fatalf("expected %+v, got %+v", wnames, fnames)
	}

	// no error should be reported from purge goroutine
	select {
	case f := <-purgec:
		t.Fatalf("unexpected purge on %q", f)
	case err = <-errc:
		t.Fatal(err)
	case <-time.After(10 * time.Millisecond):
	}
	close(stopc)
}

func TestPurgeFileHoldingOpenFileWithLock(t *testing.T) {
	dir, err := ioutil.TempDir("", "purgefile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	for i := 0; i < 10; i++ {
		var f *os.File
		f, err = os.Create(path.Join(dir, fmt.Sprintf("%d.test", i)))
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}

	// create a purge barrier at 5
	p := filepath.Join(dir, fmt.Sprintf("%d.test", 5))
	l, err := OpenFileWithLock(p, os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}

	stopc := make(chan struct{})
	errc := PurgeFile(dir, "test", 3, time.Millisecond, stopc)

	var fs []string
	for i := 0; i < 10; i++ {
		fs, err = ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}

		if len(fs) <= 5 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !reflect.DeepEqual(fs, []string{"5.test", "6.test", "7.test", "8.test", "9.test"}) {
		t.Fatalf("unexpected %v", fs)
	}

	// no error should be reported from purge goroutine
	select {
	case err = <-errc:
		t.Fatal(err)
	case <-time.After(time.Millisecond):
	}

	// remove the purge barrier
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		fs, err = ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}

		if len(fs) <= 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !reflect.DeepEqual(fs, []string{"7.test", "8.test", "9.test"}) {
		t.Fatalf("unexpected %v", fs)
	}

	// no error should be reported from purge goroutine
	select {
	case err = <-errc:
		t.Fatal(err)
	case <-time.After(time.Millisecond):
	}
	close(stopc)
}
