package raftwal

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/gyuho/db/pkg/crcutil"
	"github.com/gyuho/db/pkg/fileutil"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftwal/raftwalpb"
)

var (
	ErrMetadataConflict = errors.New("walpb: conflicting metadata found")
	ErrFileNotFound     = errors.New("walpb: file not found")
	ErrCRCMismatch      = errors.New("walpb: crc mismatch")
	ErrSnapshotMismatch = errors.New("walpb: snapshot mismatch")
	ErrSnapshotNotFound = errors.New("walpb: snapshot not found")
)

// ReadAll reads out records of the current WAL file.
//
// If opened in write mode, it must read out all records until EOF.
// Or an error will be returned.
//
// If opened in read mode, it will try to read all records if possible.
// If it cannot reach out the expected snap, it will return ErrSnapshotNotFound.
// If loaded snap doesn't match the expected one, it will return ErrSnapshotMismatch
// and all the record.
//
// After ReadAll, the WAL is ready for appending new records.
//
// (etcd wal.WAL.ReadAll)
func (w *WAL) ReadAll() (metadata []byte, hardstate raftpb.HardState, entries []raftpb.Entry, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var (
		rec   = &raftwalpb.Record{}
		dec   = w.dec
		match bool
	)
	for err = dec.decode(rec); err == nil; err = dec.decode(rec) {
		switch rec.Type {
		case raftwalpb.RECORD_TYPE_CRC:
			cv := dec.crc.Sum32()
			// if 0, the decoder is new
			if cv != 0 && rec.Validate(cv) != nil {
				hardstate.Reset()
				return nil, hardstate, nil, ErrCRCMismatch
			}
			// update the CRC of the decoder when needed
			dec.crc = crcutil.New(rec.CRC, crcTable)

		case raftwalpb.RECORD_TYPE_METADATA:
			if metadata != nil && !bytes.Equal(metadata, rec.Data) {
				hardstate.Reset()
				return nil, hardstate, nil, ErrMetadataConflict
			}
			metadata = rec.Data

		case raftwalpb.RECORD_TYPE_SNAPSHOT:
			var snap raftwalpb.Snapshot
			if err = (&snap).Unmarshal(rec.Data); err != nil {
				logger.Panicf("unmarshal should never fail (%v)", err)
			}

			if snap.Index == w.readStartSnapshot.Index {
				if snap.Term != w.readStartSnapshot.Term {
					hardstate.Reset()
					return nil, hardstate, nil, ErrSnapshotMismatch
				}
				match = true
			}

		case raftwalpb.RECORD_TYPE_ENTRY:
			var ent raftpb.Entry
			if err = (&ent).Unmarshal(rec.Data); err != nil {
				logger.Panicf("unmarshal should never fail (%v)", err)
			}
			if ent.Index > w.readStartSnapshot.Index {
				entries = append(entries[:ent.Index-w.readStartSnapshot.Index-1], ent)
			}
			w.lastIndex = ent.Index

		case raftwalpb.RECORD_TYPE_HARDSTATE:
			if err = (&hardstate).Unmarshal(rec.Data); err != nil {
				logger.Panicf("unmarshal should never fail (%v)", err)
			}

		default:
			hardstate.Reset()
			return nil, hardstate, nil, fmt.Errorf("unexpected record type %d", rec.Type)
		}
	}

	switch w.unsafeLastFile() {
	case nil:
		// no need to read out all records in read mode
		// because the last record might be partially written
		// so io.ErrUnexpectedEOF might be returned
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			hardstate.Reset()
			return nil, hardstate, nil, err
		}

	default:
		// write mode, must read all entries in write mode
		if err != io.EOF {
			hardstate.Reset()
			return nil, hardstate, nil, err
		}

		// decoder.decode returns io.EOF if it detects a zero record
		// but this zero record may be followed by non-zero records
		// from a torn write.
		//
		// Overwriting some of these non-zero records, but not all,
		// will cause CRC errors on WAL open.
		//
		// Since the records with torn-writes are never fully synced
		// to disk in the first place, it's safe to zero them out to
		// avoid any CRC mismatch errors for the new writes.
		//
		// seek relative to the origin of the file
		_, err = w.unsafeLastFile().Seek(w.dec.lastValidOffset, io.SeekStart)
		if err != nil {
			return nil, hardstate, nil, err
		}
		if err = fileutil.ZeroToEnd(w.unsafeLastFile().File); err != nil {
			return nil, hardstate, nil, err
		}
	}

	err = nil
	if !match {
		err = ErrSnapshotNotFound
	}

	// close decoder to disable reading
	if w.decoderReaderCloser != nil {
		w.decoderReaderCloser()
		w.decoderReaderCloser = nil
	}
	w.readStartSnapshot = raftwalpb.Snapshot{}
	w.metadata = metadata

	if w.unsafeLastFile() != nil { // write mode
		// create encoder to enable appends
		w.enc, err = newFileEncoder(w.unsafeLastFile().File, w.dec.crc.Sum32())
		if err != nil {
			return
		}
	}

	// done with reading
	w.dec = nil

	return metadata, hardstate, entries, err
}
