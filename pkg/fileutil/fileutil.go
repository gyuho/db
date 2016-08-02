package fileutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
)

const (
	// PrivateFileMode grants owner to read/write a file.
	PrivateFileMode = 0600

	// PrivateDirMode grants owner to make/remove files inside the directory.
	PrivateDirMode = 0700
)

// OpenToRead opens a file for reads. Make sure to close the file.
func OpenToRead(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_RDONLY, PrivateFileMode)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// OpenToOverwrite creates or opens a file for overwriting.
// Make sure to close the file.
func OpenToOverwrite(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_RDWR|os.O_TRUNC|os.O_CREATE, PrivateFileMode)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// OpenToOverwriteOnly opens a file only for overwriting.
func OpenToOverwriteOnly(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, PrivateFileMode)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// OpenToAppend opens a file for appends. If the file does not eixst, it creates one.
// Make sure to close the file.
func OpenToAppend(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_RDWR|os.O_APPEND|os.O_CREATE, PrivateFileMode)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// OpenToAppendOnly opens a file only for appends.
func OpenToAppendOnly(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, PrivateFileMode)
	if err != nil {
		return nil, err
	}
	return f, nil
}

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
//
// (etcd pkg.fileutil.TouchDirAll)
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
//
// (etcd pkg.fileutil.CreateDirAll)
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
//
// (etcd pkg.fileutil.Exist)
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
