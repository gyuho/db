package fileutil

import (
	"io"
	"os"
	"syscall"
)

// Fsync commits the current contents of the file to the disk.
// Typically it means flushing the file system's in-memory copy
// of recently written data to the disk.
//
// (etcd pkg.fileutil.Fsync)
func Fsync(f *os.File) error {
	return f.Sync()
}

// Fdatasync flushes all data buffers of a file onto the disk.
// Fsync is required to update the metadata, such as access time.
// Fsync always does two write operations: one for writing new data
// to disk. Another for updating the modification time stored in its
// inode. If the modification time is not a part of the transaction,
// syscall.Fdatasync can be used to avoid unnecessary inode disk writes.
//
// (etcd pkg.fileutil.Fdatasync)
func Fdatasync(f *os.File) error {
	return syscall.Fdatasync(int(f.Fd()))
}

// WriteSync behaves just like ioutil.WriteFile,
// but calls Sync before closing the file to guarantee that
// the data is synced if there's no error returned.
//
// (etcd pkg.ioutil.WriteAndSyncFile)
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
