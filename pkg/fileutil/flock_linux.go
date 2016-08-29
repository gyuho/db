package fileutil

import (
	"errors"
	"io"
	"os"
	"syscall"
)

// LockedFile wraps *os.File with flock syscalls.
type LockedFile struct {
	*os.File
}

// NFS fails with EBADF with syscall.Flock()
// Alternative is use Linux non-standard open file descriptor
//
// constants from /usr/include/bits/fcntl-linux.h
// https://www.gnu.org/software/libc/manual/html_node/Open-File-Description-Locks.html
const (
	F_OFD_GETLK  = 37 // specify that it should get information about a lock
	F_OFD_SETLK  = 37 // specify that it should set or clear a lock
	F_OFD_SETLKW = 38 // specify that it should set or clear a lock
	// just like the F_OFD_SETLK command, but causes the process to wait until
	// the request can be completed.
)

var (
	ErrLocked = errors.New("fileutil: file already locked")

	writeLock = syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Whence: int16(io.SeekStart), // beginning of the file
		Start:  0,
		Len:    0,
	}

	funcOpenFileWithLock            = flock
	funcOpenFileWithLockNonBlocking = flockNonBlocking
)

func init() {
	// use open file descriptor locks if the system suppoprts it
	readLock := syscall.Flock_t{Type: syscall.F_RDLCK}
	if err := syscall.FcntlFlock(0, F_OFD_GETLK, &readLock); err == nil {
		funcOpenFileWithLock = flock_OFD
		funcOpenFileWithLockNonBlocking = flock_OFD_NonBlocking
	}
}

// OpenFileWithLock opens, locks the file.
func OpenFileWithLock(fpath string, flag int, perm os.FileMode) (*LockedFile, error) {
	return funcOpenFileWithLock(fpath, flag, perm)
}

// OpenFileWithLockNonBlocking locks the file in a non-blocking way.
func OpenFileWithLockNonBlocking(fpath string, flag int, perm os.FileMode) (*LockedFile, error) {
	return funcOpenFileWithLockNonBlocking(fpath, flag, perm)
}

func flock(fpath string, flag int, perm os.FileMode) (*LockedFile, error) {
	f, err := os.OpenFile(fpath, flag, perm)
	if err != nil {
		return nil, err
	}

	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}

	return &LockedFile{f}, nil
}

func flockNonBlocking(fpath string, flag int, perm os.FileMode) (*LockedFile, error) {
	f, err := os.OpenFile(fpath, flag, perm)
	if err != nil {
		return nil, err
	}

	// non-blocking
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			err = ErrLocked
		}
		return nil, err
	}

	return &LockedFile{f}, nil
}

func flock_OFD(fpath string, flag int, perm os.FileMode) (*LockedFile, error) {
	f, err := os.OpenFile(fpath, flag, perm)
	if err != nil {
		return nil, err
	}

	wrlck := writeLock
	if err = syscall.FcntlFlock(f.Fd(), F_OFD_SETLKW, &wrlck); err != nil {
		f.Close()
		return nil, err
	}

	return &LockedFile{f}, nil
}

func flock_OFD_NonBlocking(fpath string, flag int, perm os.FileMode) (*LockedFile, error) {
	f, err := os.OpenFile(fpath, flag, perm)
	if err != nil {
		return nil, err
	}

	wrlck := writeLock
	if err = syscall.FcntlFlock(f.Fd(), F_OFD_SETLK, &wrlck); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			err = ErrLocked
		}
		return nil, err
	}

	return &LockedFile{f}, nil
}
