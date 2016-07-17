package wal

import (
	"os"
	"path/filepath"
	"time"

	"github.com/gyuho/db/fileutil"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/wal/walpb"
)

const (
	// If preallocWithExtendFile is true, it calls fallocate without FALLOC_FL_KEEP_SIZE mode,
	// which means the file size will be changed depending on the offset.
	preallocWithExtendFile = true

	// expected size of each segmented wal file
	// (actual size might be bigger than this)
	segmentSizeBytes = 64 * 1024 * 1024 // 64 MB

	// warnSyncDuration is the amount of time allotted to an fsync before
	// logging a warning
	warnSyncDuration = time.Second
)

// Create creates a WAL ready for appends, with the metadata to
// be written at the head of each WAL file. It can be retrieved
// with ReadAll.
//
// (etcd wal.Create)
func Create(dir string, metadata []byte) (*WAL, error) {
	if fileutil.DirHasFiles(dir) {
		return nil, os.ErrExist
	}

	// create temporary directory, and rename later to make it appear atomic
	tmpDir := filepath.Clean(dir) + ".tmp"
	if fileutil.ExistFileOrDir(tmpDir) {
		if err := os.RemoveAll(tmpDir); err != nil {
			return nil, err
		}
	}
	if err := fileutil.MkdirAll(tmpDir); err != nil {
		return nil, err
	}

	walPath := filepath.Join(tmpDir, getWALName(0, 0))
	f, err := fileutil.OpenFileWithLock(walPath, os.O_WRONLY|os.O_CREATE, fileutil.PrivateFileMode)
	if err != nil {
		return nil, err
	}

	// set offset to the end of file with 0 for pre-allocation
	if _, err := f.Seek(0, os.SEEK_END); err != nil {
		return nil, err
	}
	if err := fileutil.Preallocate(f.File, segmentSizeBytes, preallocWithExtendFile); err != nil {
		return nil, err
	}

	var prevCRC uint32
	w := &WAL{
		dir:         dir,
		lockedFiles: []*fileutil.LockedFile{f},
		enc:         newEncoder(f, prevCRC),
		metadata:    metadata,
	}

	// 1. encode CRC
	if err := w.UnsafeEncodeCRC(prevCRC); err != nil {
		return nil, err
	}

	// 2. encode metadata
	if err := w.UnsafeEncodeMetadata(metadata); err != nil {
		return nil, err
	}

	// 3. encode snapshot
	if err := w.UnsafeEncodeSnapshotAndFdatasync(&walpb.Snapshot{Term: 0, Index: 0}); err != nil {
		return nil, err
	}

	// Linux can:
	//
	// if err := os.RemoveAll(dir); err != nil {
	// 	return nil, err
	// }
	// if err := os.Rename(tmpDir, dir); err != nil {
	// 	return nil, err
	// }
	// w.filePipeline = newFilePipeline(dir, segmentSizeBytes)
	//
	// But some OS (windows) doesn't support renaming directory with locked files
	// (https://github.com/coreos/etcd/issues/5852)

	// close WAL to release locks, so the directory can be renamed
	if err := w.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpDir, dir); err != nil { // os.Rename won't error even if 'dir' exists
		return nil, err
	}

	// reopen and relock
	newWAL, oerr := OpenWALWrite(dir, walpb.Snapshot{})
	if oerr != nil {
		return nil, oerr
	}
	if _, _, _, err := newWAL.ReadAll(); err != nil {
		newWAL.Close()
		return nil, err
	}
	return newWAL, nil
}

// UnsafeFdatasync fsyncs the last file in the lockedFiles to the disk.
func (w *WAL) UnsafeFdatasync() error {
	if w.enc != nil {
		if err := w.enc.flush(); err != nil {
			return err
		}
	}

	st := time.Now()
	err := fileutil.Fdatasync(w.UnsafeLastFile().File)
	took := time.Since(st)

	if took > warnSyncDuration {
		logger.Warningf("fsync took too long (took %v, expected %v)", took, warnSyncDuration)
	}
	return err
}

// UnsafeEncodeCRC encodes the CRC record.
func (w *WAL) UnsafeEncodeCRC(crc uint32) error {
	return w.enc.encode(&walpb.Record{
		Type: walpb.RECORD_TYPE_CRC,
		CRC:  crc,
	})
}

// UnsafeEncodeMetadata encodes metadata to the record.
func (w *WAL) UnsafeEncodeMetadata(meatadata []byte) error {
	return w.enc.encode(&walpb.Record{
		Type: walpb.RECORD_TYPE_METADATA,
		Data: meatadata,
	})
}

// UnsafeEncodeSnapshotAndFdatasync encodes walpb.Snapshot to the record.
func (w *WAL) UnsafeEncodeSnapshotAndFdatasync(snap *walpb.Snapshot) error {
	data, err := snap.Marshal()
	if err != nil {
		return err
	}

	if err := w.enc.encode(&walpb.Record{
		Type: walpb.RECORD_TYPE_SNAPSHOT,
		Data: data,
	}); err != nil {
		return err
	}

	if w.lastIndex < snap.Index {
		// update only when snapshot is ahead of last index
		w.lastIndex = snap.Index
	}

	return w.UnsafeFdatasync()
}

// UnsafeEncodeEntry encodes raftpb.Entry to the record.
func (w *WAL) UnsafeEncodeEntry(ent *raftpb.Entry) error {
	data, err := ent.Marshal()
	if err != nil {
		return err
	}

	if err := w.enc.encode(&walpb.Record{
		Type: walpb.RECORD_TYPE_ENTRY,
		Data: data,
	}); err != nil {
		return err
	}

	w.lastIndex = ent.Index
	return nil
}

// UnsafeEncodeHardState encodes raftpb.HardState to the record.
func (w *WAL) UnsafeEncodeHardState(state *raftpb.HardState) error {
	if raftpb.IsEmptyHardState(*state) {
		return nil
	}

	data, err := state.Marshal()
	if err != nil {
		return err
	}

	if err := w.enc.encode(&walpb.Record{
		Type: walpb.RECORD_TYPE_HARDSTATE,
		Data: data,
	}); err != nil {
		return err
	}

	w.hardState = *state
	return nil
}

// UnsafeCutCurrent closes currently written file.
// It first creates a temporary WAL file to write necessary headers onto.
// And atomically rename the temporary WAL file to a WAL file.
func (w *WAL) UnsafeCutCurrent() error {
	// set offset to current
	offset, err := w.UnsafeLastFile().Seek(0, os.SEEK_CUR)
	if err != nil {
		return err
	}

	// truncate to avoid wasting space with early cut
	if err = w.UnsafeLastFile().Truncate(offset); err != nil {
		return err
	}

	// fsync to the disk
	if err = w.UnsafeFdatasync(); err != nil {
		return err
	}

	// next WAL file with index + 1
	walPath := filepath.Join(w.dir, getWALName(w.UnsafeLastFileSeq()+1, w.lastIndex+1))

	// open a temporary file to write a new data
	newLastTmpFile, err := w.filePipeline.Open()
	if err != nil {
		return err
	}
	w.lockedFiles = append(w.lockedFiles, newLastTmpFile)

	// update encoder with the newly-appended last file
	prevCRC := w.enc.crc.Sum32()
	w.enc = newEncoder(w.UnsafeLastFile(), prevCRC)

	// 1. update CRC
	if err = w.UnsafeEncodeCRC(prevCRC); err != nil {
		return err
	}

	// 2. write metadata
	if err = w.UnsafeEncodeMetadata(w.metadata); err != nil {
		return err
	}

	// 3. write hard state
	if err = w.UnsafeEncodeHardState(&w.hardState); err != nil {
		return err
	}

	// fsync the last temporary file to the disk
	if err = w.UnsafeFdatasync(); err != nil {
		return err
	}

	// set offset to current, because there were writes
	offset, err = w.UnsafeLastFile().Seek(0, os.SEEK_CUR)
	if err != nil {
		return err
	}

	// rename the file to WAL name atomically
	if err = os.Rename(newLastTmpFile.Name(), walPath); err != nil {
		return err
	}

	// release the lock, flush buffer
	if err = newLastTmpFile.Close(); err != nil {
		return err
	}

	// create a new locked file for appends
	newLastTmpFile, err = fileutil.OpenFileWithLock(walPath, os.O_WRONLY, fileutil.PrivateFileMode)
	if err != nil {
		return err
	}

	// move(set) beginning of the file(os.SEEK_SET) to offset, because there were writes
	if _, err = newLastTmpFile.Seek(offset, os.SEEK_SET); err != nil { // 0, os.SEEK_SET: seek relative to the origin(beginning) of the file
		return err
	}

	// update the last file
	w.lockedFiles[len(w.lockedFiles)-1] = newLastTmpFile

	// update CRC from the newly-locked file
	prevCRC = w.enc.crc.Sum32()

	// update the encoder with newly-locked file
	w.enc = newEncoder(w.UnsafeLastFile(), prevCRC)

	logger.Infof("created %q", walPath)
	return nil
}

// Save stores the raftpb.HardState with entries.
func (w *WAL) Save(st raftpb.HardState, ents []raftpb.Entry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if raftpb.IsEmptyHardState(st) && len(ents) == 0 {
		return nil
	}

	needFsync := raftpb.HardStateContainUpdates(w.hardState, st, len(ents))

	// write entries
	for i := range ents {
		if err := w.UnsafeEncodeEntry(&ents[i]); err != nil {
			return err
		}
	}

	// write hard state
	if err := w.UnsafeEncodeHardState(&st); err != nil {
		return err
	}

	// seek the current location, and get the offset
	curOffset, err := w.UnsafeLastFile().Seek(0, os.SEEK_CUR)
	if err != nil {
		return err
	}

	if curOffset < segmentSizeBytes { // no need to cut
		if needFsync {
			return w.UnsafeFdatasync()
		}
		return nil
	}

	return w.UnsafeCutCurrent()
}
