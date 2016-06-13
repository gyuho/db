package wal

import (
	"path/filepath"
	"sync"

	"github.com/gyuho/distdb/fileutil"
	"github.com/gyuho/distdb/raftpb"
	"github.com/gyuho/distdb/walpb"
)

// WAL is the logical representation of the stable storage.
// WAL is either in read-only or append-only mode.
// A newly created WAL file is append-only.
// A just-opened WAL file is read-only, and ready for appending
// after reading out all the previous records.
type WAL struct {
	mu sync.Mutex

	// dir is the location of all underlying WAL files.
	dir          string
	filePipeline *filePipeline
	lockedFiles  []*fileutil.LockedFile

	enc *encoder

	// metadata is recorded at the head of each WAL file.
	metadata []byte

	// hardState is recorded at the haed of each WAL file.
	hardState raftpb.HardState

	// lastIndex is the index of the last entry saved to WAL.
	lastIndex uint64

	dec                 *decoder
	decoderReaderCloser func() error
	readStartSnapshot   walpb.Snapshot
}

// Lock locks the WAL.
func (w *WAL) Lock() { w.mu.Lock() }

// Unlock unlocks the WAL.
func (w *WAL) Unlock() { w.mu.Unlock() }

// UnsafeLastFile returns the last file in the lockedFiles.
func (w *WAL) UnsafeLastFile() *fileutil.LockedFile {
	n := len(w.lockedFiles)
	if n > 0 {
		return w.lockedFiles[n-1]
	}
	return nil
}

// UnsafeLastFileSeq returns the sequence number of the
// last file in the lockedFiles.
func (w *WAL) UnsafeLastFileSeq() uint64 {
	f := w.UnsafeLastFile()
	if f == nil {
		return 0
	}

	seq, _, err := parseWALName(filepath.Base(f.Name()))
	if err != nil {
		logger.Fatalf("failed to parse WAL file %q (%v)", f.Name(), err)
	}
	return seq
}

// Close closes all filePipeline and lockedFiles.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.filePipeline != nil {
		w.filePipeline.Close()
		w.filePipeline = nil
	}

	// fsync
	if w.UnsafeLastFile() != nil {
		if err := w.UnsafeFdatasync(); err != nil {
			return err
		}
	}

	for _, lf := range w.lockedFiles {
		if lf == nil {
			continue
		}
		if err := lf.Close(); err != nil {
			logger.Fatalf("failed to unlock the LockedFile %q during closing (%v)", lf.Name(), err)
		}
	}

	return nil
}

// ReleaseLocks releases locks whose index is smaller than the given,
// excep the largest one among those.
//
// For example, if WAL is holding locks 1,2,3,4,5, then ReleaseLocks(4)
// releases locks for  1,2 (not 3). ReleaseLocks(5) releases 1,2,3.
func (w *WAL) ReleaseLocks(idx uint64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var (
		lastToExclude int
		found         bool
	)
	for i, f := range w.lockedFiles {
		_, lockedIndex, err := parseWALName(filepath.Base(f.Name()))
		if err != nil {
			return err
		}

		if lockedIndex >= idx {
			lastToExclude = i - 1
			found = true
			break
		}
	}

	// if not found, release up to second-last
	if !found && len(w.lockedFiles) > 0 {
		lastToExclude = len(w.lockedFiles) - 1
	}

	if lastToExclude <= 0 {
		return nil
	}

	for i := 0; i < lastToExclude; i++ {
		if w.lockedFiles[i] == nil {
			continue
		}
		w.lockedFiles[i].Close()
	}
	w.lockedFiles = w.lockedFiles[lastToExclude:]

	return nil
}
