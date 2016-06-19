package fileutil

import (
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestLockAndUnlock(t *testing.T) {
	f, err := ioutil.TempFile("", "lock")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	defer func() {
		if err = os.Remove(f.Name()); err != nil {
			t.Fatal(err)
		}
	}()

	// lock the file
	l, err := LockFile(f.Name(), os.O_WRONLY, PrivateFileMode)
	if err != nil {
		t.Fatal(err)
	}

	// try lock a locked file
	if _, err = LockFileNonBlocking(f.Name(), os.O_WRONLY, PrivateFileMode); err != ErrLocked {
		t.Fatal(err)
	}

	// unlock the file
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}

	// try lock the unlocked file
	l2, err := LockFileNonBlocking(f.Name(), os.O_WRONLY, PrivateFileMode)
	if err != nil {
		t.Fatal(err)
	}

	// double-lock should block
	done := make(chan struct{}, 1)
	go func() {
		var bl *LockedFile
		bl, err = LockFile(f.Name(), os.O_WRONLY, PrivateFileMode)
		if err != nil {
			t.Fatal(err)
		}
		done <- struct{}{}
		if err = bl.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	select {
	case <-done:
		t.Fatal("unexpected unblocking")
	case <-time.After(100 * time.Millisecond):
	}

	// unlock l2
	if err = l2.Close(); err != nil {
		t.Fatal(err)
	}

	// previously-blocked one should get that lock now
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected blocking")
	}
}
