package raftwal

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/gyuho/db/pkg/fileutil"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftwal/raftwalpb"
)

// WAL is the logical representation of the stable storage.
// WAL is either in read-only or append-only mode.
// A newly created WAL file is append-only.
// A just-opened WAL file is read-only, and ready for appending
// after reading out all the previous records.
type WAL struct {
	mu sync.Mutex

	// dir is the location of all underlying WAL files.
	dir string

	// dirFile is a fd for the wal directory for syncing on Rename
	dirFile *os.File

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
	readStartSnapshot   raftwalpb.Snapshot
}

// Lock locks the WAL.
func (w *WAL) Lock() { w.mu.Lock() }

// Unlock unlocks the WAL.
func (w *WAL) Unlock() { w.mu.Unlock() }

// unsafeLastFile returns the last file in the lockedFiles.
//
// (etcd wal.WAL.tail)
func (w *WAL) unsafeLastFile() *fileutil.LockedFile {
	if len(w.lockedFiles) > 0 {
		return w.lockedFiles[len(w.lockedFiles)-1]
	}
	return nil
}

// unsafeLastFileSeq returns the sequence number of the
// last file in the lockedFiles.
//
// (etcd wal.WAL.seq)
func (w *WAL) unsafeLastFileSeq() uint64 {
	f := w.unsafeLastFile()
	if f == nil {
		return 0
	}

	seq, _, err := parseWALName(filepath.Base(f.Name()))
	if err != nil {
		logger.Fatalf("failed to parse %q (%v)", f.Name(), err)
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
	if w.unsafeLastFile() != nil {
		if err := w.unsafeFdatasync(); err != nil {
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

	return w.dirFile.Close()
}

// openLastWALFile opens the last WAL file for read and write.
//
// (etcd wal.openLast)
func openLastWALFile(dir string) (*fileutil.LockedFile, error) {
	wnames, err := readWALNames(dir)
	if err != nil {
		return nil, err
	}
	fpath := filepath.Join(dir, wnames[len(wnames)-1])
	return fileutil.OpenFileWithLock(fpath, os.O_RDWR, fileutil.PrivateFileMode)
}

// openWAL opens a WAL file with given snapshot.
//
// (etcd wal.openAtIndex)
func openWAL(dir string, snap raftwalpb.Snapshot, write bool) (*WAL, error) {
	names, err := readWALNames(dir)
	if err != nil {
		return nil, err
	}

	lastWALIdx := searchLastWALIndex(names, snap.Index)
	if lastWALIdx < 0 || !areWALNamesSorted(names[lastWALIdx:]) {
		return nil, ErrFileNotFound
	}
	names = names[lastWALIdx:]

	// open WAL files
	var (
		readClosers []io.ReadCloser
		readers     []io.Reader
		lockedFiles []*fileutil.LockedFile
	)
	for _, name := range names {
		fpath := filepath.Join(dir, name)
		switch write {
		case true:
			f, err := fileutil.OpenFileWithLockNonBlocking(fpath, os.O_RDWR, fileutil.PrivateFileMode)
			if err != nil {
				closeAll(readClosers...)
				return nil, err
			}
			readClosers = append(readClosers, f)
			lockedFiles = append(lockedFiles, f)

		case false:
			f, err := os.OpenFile(fpath, os.O_RDONLY, fileutil.PrivateFileMode)
			if err != nil {
				closeAll(readClosers...)
				return nil, err
			}
			readClosers = append(readClosers, f)
			lockedFiles = append(lockedFiles, nil)
		}
		readers = append(readers, readClosers[len(readClosers)-1])
	}
	closeFunc := func() error {
		return closeAll(readClosers...)
	}

	w := &WAL{
		dir:                 dir,
		lockedFiles:         lockedFiles,
		dec:                 newDecoder(readers...),
		decoderReaderCloser: closeFunc,
		readStartSnapshot:   snap,
	}

	if write {
		// Write reuses the file descriptors from read.
		// Don't close, so that WAL can append without releasing flocks.
		w.decoderReaderCloser = nil
		if _, _, err := parseWALName(filepath.Base(w.unsafeLastFile().Name())); err != nil {
			closeFunc()
			return nil, err
		}
		w.filePipeline = newFilePipeline(dir, segmentSizeBytes)
	}
	return w, nil
}

// OpenWALWrite opens the WAL file at the given snapshot for writes.
// The snap must have had been stored in the WAL, or the following ReadAll fails.
// The returned WAL is ready for reads, and appends only after reading out
// all of its previous records.
// The first record will be the one after the given snapshot.
//
// (etcd wal.Open)
func OpenWALWrite(dir string, snap raftwalpb.Snapshot) (*WAL, error) {
	w, err := openWAL(dir, snap, true)
	if err != nil {
		return nil, err
	}
	if w.dirFile, err = fileutil.OpenDir(w.dir); err != nil {
		return nil, err
	}
	return w, nil
}

// OpenWALRead opens the WAL file for reads.
//
// (etcd wal.OpenForRead)
func OpenWALRead(dir string, snap raftwalpb.Snapshot) (*WAL, error) {
	return openWAL(dir, snap, false)
}

// ReleaseLocks releases locks whose index is smaller than the given,
// excep the largest one among those.
//
// For example, if WAL is holding locks 1,2,3,4,5, then ReleaseLocks(4)
// releases locks for  1,2 (not 3). ReleaseLocks(5) releases 1,2,3.
//
// (etcd wal.WAL.ReleaseLockTo)
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
	if _, err = f.Seek(0, io.SeekEnd); err != nil {
		return nil, err
	}
	if err = fileutil.Preallocate(f.File, segmentSizeBytes, preallocWithExtendFile); err != nil {
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
	if err = w.unsafeEncodeCRC(prevCRC); err != nil {
		return nil, err
	}

	// 2. encode metadata
	if err = w.unsafeEncodeMetadata(metadata); err != nil {
		return nil, err
	}

	// 3. encode snapshot
	if err = w.UnsafeEncodeSnapshotAndFdatasync(&raftwalpb.Snapshot{Term: 0, Index: 0}); err != nil {
		return nil, err
	}

	if w, err = w.renameWAL(tmpDir); err != nil {
		return nil, err
	}

	// directory was renamed; sync parent dir to persist rename
	pdir, perr := fileutil.OpenDir(path.Dir(w.dir))
	if perr != nil {
		return nil, perr
	}
	if perr = fileutil.Fsync(pdir); perr != nil {
		return nil, perr
	}
	if perr = pdir.Close(); err != nil {
		return nil, perr
	}
	return w, nil
}

// unsafeFdatasync fsyncs the last file in the lockedFiles to the disk.
//
// (etcd wal.WAL.sync)
func (w *WAL) unsafeFdatasync() error {
	if w.enc != nil {
		if err := w.enc.flush(); err != nil {
			return err
		}
	}

	st := time.Now()
	err := fileutil.Fdatasync(w.unsafeLastFile().File)
	took := time.Since(st)

	if took > warnSyncDuration {
		logger.Warningf("fsync took too long (took %v, expected %v)", took, warnSyncDuration)
	}
	return err
}

// unsafeEncodeCRC encodes the CRC record.
//
// (etcd wal.WAL.saveCrc)
func (w *WAL) unsafeEncodeCRC(crc uint32) error {
	return w.enc.encode(&raftwalpb.Record{
		Type: raftwalpb.RECORD_TYPE_CRC,
		CRC:  crc,
	})
}

// unsafeEncodeMetadata encodes metadata to the record.
func (w *WAL) unsafeEncodeMetadata(meatadata []byte) error {
	return w.enc.encode(&raftwalpb.Record{
		Type: raftwalpb.RECORD_TYPE_METADATA,
		Data: meatadata,
	})
}

// UnsafeEncodeSnapshotAndFdatasync encodes raftwalpb.Snapshot to the record.
//
// (etcd wal.WAL.SaveSnapshot)
func (w *WAL) UnsafeEncodeSnapshotAndFdatasync(snap *raftwalpb.Snapshot) error {
	data, err := snap.Marshal()
	if err != nil {
		return err
	}
	if err := w.enc.encode(&raftwalpb.Record{
		Type: raftwalpb.RECORD_TYPE_SNAPSHOT,
		Data: data,
	}); err != nil {
		return err
	}

	if w.lastIndex < snap.Index {
		// update only when snapshot is ahead of last index
		w.lastIndex = snap.Index
	}
	return w.unsafeFdatasync()
}

// unsafeEncodeEntry encodes raftpb.Entry to the record.
//
// (etcd wal.WAL.saveEntry)
func (w *WAL) unsafeEncodeEntry(ent *raftpb.Entry) error {
	data, err := ent.Marshal()
	if err != nil {
		return err
	}

	if err := w.enc.encode(&raftwalpb.Record{
		Type: raftwalpb.RECORD_TYPE_ENTRY,
		Data: data,
	}); err != nil {
		return err
	}
	w.lastIndex = ent.Index
	return nil
}

// unsafeEncodeHardState encodes raftpb.HardState to the record.
//
// (etcd wal.WAL.saveState)
func (w *WAL) unsafeEncodeHardState(state *raftpb.HardState) error {
	if raftpb.IsEmptyHardState(*state) {
		return nil
	}
	w.hardState = *state

	data, err := state.Marshal()
	if err != nil {
		return err
	}

	return w.enc.encode(&raftwalpb.Record{
		Type: raftwalpb.RECORD_TYPE_HARDSTATE,
		Data: data,
	})
}

// unsafeCutCurrent closes currently written file.
// It first creates a temporary WAL file to write necessary headers onto.
// And atomically rename the temporary WAL file to a WAL file.
//
// (etcd wal.WAL.cut)
func (w *WAL) unsafeCutCurrent() error {
	// set offset to current
	offset, err := w.unsafeLastFile().Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	// truncate to avoid wasting space with early cut
	if err = w.unsafeLastFile().Truncate(offset); err != nil {
		return err
	}

	// fsync to the disk
	if err = w.unsafeFdatasync(); err != nil {
		return err
	}

	// next WAL file with index + 1
	walPath := filepath.Join(w.dir, getWALName(w.unsafeLastFileSeq()+1, w.lastIndex+1))

	// open a temporary file to write a new data
	newLastTmpFile, err := w.filePipeline.Open()
	if err != nil {
		return err
	}
	w.lockedFiles = append(w.lockedFiles, newLastTmpFile)

	// update encoder with the newly-appended last file
	prevCRC := w.enc.crc.Sum32()
	w.enc = newEncoder(w.unsafeLastFile(), prevCRC)

	// 1. update CRC
	if err = w.unsafeEncodeCRC(prevCRC); err != nil {
		return err
	}

	// 2. write metadata
	if err = w.unsafeEncodeMetadata(w.metadata); err != nil {
		return err
	}

	// 3. write hard state
	if err = w.unsafeEncodeHardState(&w.hardState); err != nil {
		return err
	}

	// fsync the last temporary file to the disk
	if err = w.unsafeFdatasync(); err != nil {
		return err
	}

	// set offset to current, because there were writes
	offset, err = w.unsafeLastFile().Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	// rename the file to WAL name atomically
	if err = os.Rename(newLastTmpFile.Name(), walPath); err != nil {
		return err
	}
	if err = fileutil.Fsync(w.dirFile); err != nil {
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

	// move(set) beginning of the file(io.SeekStart) to offset, because there were writes
	if _, err = newLastTmpFile.Seek(offset, io.SeekStart); err != nil { // 0, io.SeekStart: seek relative to the origin(beginning) of the file
		return err
	}

	// update the last file
	w.lockedFiles[len(w.lockedFiles)-1] = newLastTmpFile

	// update CRC from the newly-locked file
	prevCRC = w.enc.crc.Sum32()

	// update the encoder with newly-locked file
	w.enc = newEncoder(w.unsafeLastFile(), prevCRC)

	logger.Infof("created %q", walPath)
	return nil
}

// Save stores the raftpb.HardState with entries.
//
// (etcd wal.WAL.Save)
func (w *WAL) Save(st raftpb.HardState, ents []raftpb.Entry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if raftpb.IsEmptyHardState(st) && len(ents) == 0 {
		return nil
	}

	needFsync := raftpb.HardStateContainUpdates(w.hardState, st, len(ents))

	// write entries
	for i := range ents {
		if err := w.unsafeEncodeEntry(&ents[i]); err != nil {
			return err
		}
	}

	// write hard state
	if err := w.unsafeEncodeHardState(&st); err != nil {
		return err
	}

	// seek the current location, and get the offset
	curOffset, err := w.unsafeLastFile().Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	if curOffset < segmentSizeBytes { // no need to cut
		if needFsync {
			return w.unsafeFdatasync()
		}
		return nil
	}
	return w.unsafeCutCurrent()
}
