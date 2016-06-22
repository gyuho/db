package fileutil

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

const (
	// PrivateFileMode grants owner to read/write a file.
	PrivateFileMode = 0600

	// PrivateDirMode grants owner to make/remove files inside the directory.
	PrivateDirMode = 0700
)

// DirWritable returns nil if dir is writable.
func DirWritable(dir string) error {
	f := filepath.Join(dir, ".touch")
	if err := ioutil.WriteFile(f, []byte(""), PrivateFileMode); err != nil {
		return err
	}
	return os.Remove(f)
}

// ReadDir returns the filenames in the given directory in sorted order.
func ReadDir(dir string) ([]string, error) {
	d, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer d.Close()

	ns, err := d.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	sort.Strings(ns)

	return ns, nil
}

// MkdirAll runs os.MkdirAll with writable check.
func MkdirAll(dir string) error {
	// If path is already a directory, MkdirAll does nothing
	// and returns nil.
	err := os.MkdirAll(dir, PrivateDirMode)
	if err != nil {
		// if mkdirAll("a/text") and "text" is not
		// a directory, this will return syscall.ENOTDIR
		return err
	}
	return DirWritable(dir)
}

// MkdirAllEmpty is similar to MkdirAll but returns error
// if the deepest directory was not empty.
func MkdirAllEmpty(dir string) error {
	err := MkdirAll(dir)
	if err == nil {
		var ns []string
		ns, err = ReadDir(dir)
		if err != nil {
			return err
		}
		if len(ns) != 0 {
			err = fmt.Errorf("expected %q to be empty, got %q", dir, ns)
		}
	}
	return err
}

// ExistFileOrDir returns true if the file or directory exists.
func ExistFileOrDir(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

// DirHasFiles returns true only when the directory exists
// and it is non-empty.
func DirHasFiles(dir string) bool {
	ns, err := ReadDir(dir)
	if err != nil {
		return false
	}
	return len(ns) != 0
}

// Fsync commits the current contents of the file to the disk.
// Typically it means flushing the file system's in-memory copy
// of recently written data to the disk.
func Fsync(f *os.File) error {
	return f.Sync()
}

// Fdatasync flushes all data buffers of a file onto the disk.
// Fsync is required to update the metadata, such as access time.
// Fsync always does two write operations: one for writing new data
// to disk. Another for updating the modification time stored in its
// inode. If the modification time is not a part of the transaction,
// syscall.Fdatasync can be used to avoid unnecessary inode disk writes.
func Fdatasync(f *os.File) error {
	return syscall.Fdatasync(int(f.Fd()))
}

// WriteSync behaves just like ioutil.WriteFile,
// but calls Sync before closing the file to guarantee that
// the data is synced if there's no error returned.
func WriteSync(fpath string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	n, err := f.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}

	if err == nil {
		err = f.Sync()
	}

	if e := f.Close(); err == nil {
		err = e
	}
	return err
}
