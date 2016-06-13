package wal

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gyuho/distdb/fileutil"
)

// filePipeline pipelines disk space allocation.
type filePipeline struct {
	// dir is the directory to put files.
	dir string

	// size of files to make in bytes.
	size int64

	// count is the number of files generated.
	count int

	lockedFileCh chan *fileutil.LockedFile
	errc         chan error
	donec        chan struct{}
}

func newFilePipeline(dir string, size int64) *filePipeline {
	fp := &filePipeline{
		dir:          dir,
		size:         size,
		count:        0,
		lockedFileCh: make(chan *fileutil.LockedFile),
		errc:         make(chan error, 1),
		donec:        make(chan struct{}),
	}
	go fp.run()
	return fp
}

const (
	privateFileMode = 0600

	// owners can make/remove files in this directory
	privateDirMode = 0700
)

func (fp *filePipeline) alloc() (f *fileutil.LockedFile, err error) {
	fpath := filepath.Join(fp.dir, fmt.Sprintf("%d.tmp", fp.count%2)) // to make it different than previous one
	if f, err = fileutil.LockFile(fpath, os.O_CREATE|os.O_WRONLY, privateFileMode); err != nil {
		return nil, err
	}

	extendFile := true
	if err = fileutil.Preallocate(f.File, fp.size, extendFile); err != nil {
		logger.Errorf("failed to allocate space when creating new WAL %q (%v)", fpath, err)
		f.Close()
		return nil, err
	}

	fp.count++
	return f, nil
}

// Open returns a fresh file ready for writes.
// Rename the file before calling this or duplicate Open
// will trigger file collisions.
func (fp *filePipeline) Open() (f *fileutil.LockedFile, err error) {
	select {
	case f = <-fp.lockedFileCh:
	case err = <-fp.errc:
	}
	return
}

func (fp *filePipeline) Close() error {
	close(fp.donec)
	return <-fp.errc
}

func (fp *filePipeline) run() {
	defer close(fp.errc)

	for {
		f, err := fp.alloc()
		if err != nil {
			fp.errc <- err
			return
		}

		select {
		case fp.lockedFileCh <- f:
		case <-fp.donec:
			os.Remove(f.Name())
			f.Close()
			return
		}
	}
}
