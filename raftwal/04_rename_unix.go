// +build !windows

package raftwal

import "os"

func (w *WAL) renameWAL(tmpDir string) (*WAL, error) {
	// On non-Windows platforms, hold the lock while renaming. Releasing
	// the lock and trying to reacquire it quickly can be flaky because
	// it's possible the process will fork to spawn a process while this is
	// happening. The fds are set up as close-on-exec by the Go runtime,
	// but there is a window between the fork and the exec where another
	// process holds the lock.
	// (https://github.com/coreos/etcd/pull/6269)
	//
	if err := os.RemoveAll(w.dir); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpDir, w.dir); err != nil {
		return nil, err
	}
	w.filePipeline = newFilePipeline(w.dir, segmentSizeBytes)
	return w, nil
}
