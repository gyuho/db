package raftwal

import (
	"os"

	"github.com/gyuho/db/pkg/fileutil"
	"github.com/gyuho/db/raftwal/raftwalpb"
)

func (w *WAL) renameWAL(tmpDir string) (*WAL, error) {
	// But some OS (windows) doesn't support renaming directory with locked files.
	// Windoes needs to close the WAL to release the locks so the directory can be renamed.
	// (https://github.com/coreos/etcd/issues/5852)
	// (https://github.com/coreos/etcd/pull/6269)

	// close WAL to release locks, so the directory can be renamed
	if err := w.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpDir, dir); err != nil { // os.Rename won't error even if 'dir' exists
		return nil, err
	}

	// reopen and relock
	newWAL, oerr := OpenWALWrite(dir, raftwalpb.Snapshot{})
	if oerr != nil {
		return nil, oerr
	}
	if _, _, _, err := newWAL.ReadAll(); err != nil {
		newWAL.Close()
		return nil, err
	}

	// windows expects a writeable file for fsync
	df, derr := os.OpenFile(w.dir, os.O_WRONLY, fileutil.PrivateFileMode)
	if derr != nil {
		return nil, derr
	}
	w.dirFile = df

	return newWAL, nil
}
