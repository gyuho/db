package raftwal

import (
	"bufio"
	"hash"
	"io"
	"sync"

	"github.com/gyuho/db/pkg/crcutil"
	"github.com/gyuho/db/raftwal/raftwalpb"
)

type decoder struct {
	mu              sync.Mutex
	crc             hash.Hash32
	bufioReaders    []*bufio.Reader
	lastValidOffset int64
}

func newDecoder(r ...io.Reader) *decoder {
	readers := make([]*bufio.Reader, len(r))
	for i := range r {
		readers[i] = bufio.NewReader(r[i])
	}
	return &decoder{
		crc:          crcutil.New(0, crcTable),
		bufioReaders: readers,
	}
}

func (d *decoder) decode(rec *raftwalpb.Record) error {
	rec.Reset()
	d.mu.Lock()
	err := d.decodeRecord(rec)
	d.mu.Unlock()
	return err
}

func (d *decoder) decodeRecord(rec *raftwalpb.Record) error {
	if len(d.bufioReaders) == 0 {
		return io.EOF
	}

	headerN, err := readHeaderN(d.bufioReaders[0])
	switch {
	case err == io.EOF || (err == nil && headerN == 0):
		// end of file, or end of preallocated space
		d.bufioReaders = d.bufioReaders[1:]
		if len(d.bufioReaders) == 0 {
			return io.EOF
		}
		d.lastValidOffset = 0
		return d.decodeRecord(rec)

	case err != nil:
		return err
	}

	dataN, padBytesN := decodeLen(headerN)
	data := make([]byte, dataN+padBytesN)
	if _, err = io.ReadFull(d.bufioReaders[0], data); err != nil {
		// ReadFull returns io.EOF only when no bytes were read.
		// decoder should treat this as io.ErrUnexpectedEOF
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}

	if err = rec.Unmarshal(data[:dataN:dataN]); err != nil {
		if d.isTornEntry(data) {
			return io.ErrUnexpectedEOF
		}
		return err
	}

	if rec.Type != raftwalpb.RECORD_TYPE_CRC {
		d.crc.Write(rec.Data)
		if err = rec.Validate(d.crc.Sum32()); err != nil {
			if d.isTornEntry(data) {
				return io.ErrUnexpectedEOF
			}
			return err
		}
	}

	// record is considered valid
	// update the last valid offset to the end of record
	d.lastValidOffset += dataN + padBytesN + byteBitN
	return nil
}

// isTornEntry returns true when the last entry of the WAL
// was partially written and corrupted because of a torn write.
func (d *decoder) isTornEntry(data []byte) bool {
	if len(d.bufioReaders) != 1 { // not the last leader
		return false
	}

	var (
		dataN  = len(data)
		fIdx   = d.lastValidOffset + byteBitN
		chunks [][]byte

		cur int
	)
	for cur < dataN {
		chunkN := int(minSectorSize - (fIdx % minSectorSize))
		if chunkN > dataN-cur { // to prevent index out of bound
			chunkN = dataN - cur
		}

		endIdx := cur + chunkN
		chunks = append(chunks, data[cur:endIdx])

		fIdx += int64(chunkN)
		cur += chunkN
	}

	// if any data in the sector chunk is ALL 0,
	// it's a torn write
	for _, sector := range chunks {
		allZero := true
		for _, b := range sector {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero { // torn-write!
			return true
		}
	}
	return false
}
