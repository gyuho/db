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

func TestPurgeFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "purgefile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	for i := 0; i < 5; i++ {
		var f *os.File
		f, err = os.Create(filepath.Join(dir, fmt.Sprintf("%d.test", i)))
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}

	stopc := make(chan struct{})

	// keep at most 3 most recent files
	errc := PurgeFile(dir, "test", 3, time.Millisecond, stopc)

	// create 5 more files
	for i := 5; i < 10; i++ {
		var f *os.File
		f, err = os.Create(filepath.Join(dir, fmt.Sprintf("%d.test", i)))
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		time.Sleep(10 * time.Millisecond)
	}

	// purge goroutine should 7 out of 10 files keeping the 3 most recent ones
	var fs []string
	for i := 0; i < 30; i++ {
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

func TestPurgeFileHoldingLockFile(t *testing.T) {
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
	l, err := LockFile(p, os.O_WRONLY, 0600)
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
