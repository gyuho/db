package raftsnap

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gyuho/db/pkg/fileutil"
)

// SaveDB saves snapshot of the database from the given reader. It
// guarantees the save operation is atomic.
//
// (etcd snap.Snapshotter.SaveDBFrom)
func (s *Snapshotter) SaveDB(r io.Reader, id uint64) (int64, error) {
	f, err := ioutil.TempFile(s.dir, "tmp")
	if err != nil {
		return 0, err
	}

	var n int64
	n, err = io.Copy(f, r)
	if err == nil {
		err = fileutil.Fsync(f)
	}
	f.Close()
	if err != nil {
		os.Remove(f.Name())
		return n, err
	}

	fn := filepath.Join(s.dir, fmt.Sprintf("%016x.snap.db", id))
	if fileutil.ExistFileOrDir(fn) {
		os.Remove(f.Name())
		return n, nil
	}

	err = os.Rename(f.Name(), fn)
	if err != nil {
		os.Remove(f.Name())
		return n, err
	}

	logger.Infof("saved database snapshot to disk [total bytes: %d]", n)
	return n, nil
}

// DBFilePath returns the file path for the snapshot of the database with
// given id. If the snapshot does not exist, it returns error.
//
// (etcd snap.Snapshotter.DBFilePath)
func (s *Snapshotter) DBFilePath(id uint64) (string, error) {
	fns, err := fileutil.ReadDir(s.dir)
	if err != nil {
		return "", err
	}

	wfn := fmt.Sprintf("%016x.snap.db", id)
	for _, fn := range fns {
		if fn == wfn {
			return filepath.Join(s.dir, fn), nil
		}
	}
	return "", ErrNoSnapshotDBFile
}
