package wal

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/gyuho/distdb/crc"
	"github.com/gyuho/distdb/raftpb"
	"github.com/gyuho/distdb/walpb"
)

var (
	ErrMetadataConflict = errors.New("wal: conflicting metadata found")
	ErrFileNotFound     = errors.New("wal: file not found")
	ErrCRCMismatch      = errors.New("wal: crc mismatch")
	ErrSnapshotMismatch = errors.New("wal: snapshot mismatch")
	ErrSnapshotNotFound = errors.New("wal: snapshot not found")
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
func (w *WAL) ReadAll() (metadata []byte, hardstate raftpb.HardState, entries []raftpb.Entry, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var (
		rec   = &walpb.Record{}
		dec   = w.dec
		match bool
	)
	for err = dec.decode(rec); err == nil; err = dec.decode(rec) {
		switch rec.Type {
		case walpb.RECORD_TYPE_CRC:
			cv := dec.crc.Sum32()
			// if 0, the decoder is new
			if cv != 0 && rec.Validate(cv) != nil {
				hardstate.Reset()
				return nil, hardstate, nil, ErrCRCMismatch
			}
			// update the CRC of the decoder when needed
			dec.crc = crc.New(rec.CRC, crcTable)

		case walpb.RECORD_TYPE_METADATA:
			if metadata != nil && !bytes.Equal(metadata, rec.Data) {
				hardstate.Reset()
				return nil, hardstate, nil, ErrMetadataConflict
			}
			metadata = rec.Data

		case walpb.RECORD_TYPE_SNAPSHOT:
			var snap walpb.Snapshot
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

		case walpb.RECORD_TYPE_ENTRY:
			var ent raftpb.Entry
			if err = (&ent).Unmarshal(rec.Data); err != nil {
				logger.Panicf("unmarshal should never fail (%v)", err)
			}
			if ent.Index > w.readStartSnapshot.Index {
				entries = append(entries[:ent.Index-w.readStartSnapshot.Index-1], ent)
			}
			w.lastIndex = ent.Index

		case walpb.RECORD_TYPE_HARDSTATE:
			if err = (&hardstate).Unmarshal(rec.Data); err != nil {
				logger.Panicf("unmarshal should never fail (%v)", err)
			}

		default:
			hardstate.Reset()
			return nil, hardstate, nil, fmt.Errorf("unexpected record type %d", rec.Type)
		}
	}

	switch w.UnsafeLastFile() {
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
	w.readStartSnapshot = walpb.Snapshot{}
	w.metadata = metadata

	if w.UnsafeLastFile() != nil { // write mode
		// set offset with seek relative to the origin of the file
		_, err = w.UnsafeLastFile().Seek(w.dec.lastValidOffset, os.SEEK_SET)

		// create encoder to enable appends
		w.enc = newEncoder(w.UnsafeLastFile(), w.dec.crc.Sum32())
	}

	// done with reading
	w.dec = nil

	return metadata, hardstate, entries, err
}
