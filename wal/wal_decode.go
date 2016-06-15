package wal

import (
	"bufio"
	"hash"
	"io"
	"sync"

	"github.com/gyuho/distdb/crc"
	"github.com/gyuho/distdb/walpb"
)

type decoder struct {
	mu sync.Mutex

	crc hash.Hash32

	bufioReaders []*bufio.Reader

	// lastValidOffset is the file offset
	// following the last valid decoded record
	lastValidOffset int64
}

func newDecoder(r ...io.Reader) *decoder {
	readers := make([]*bufio.Reader, len(r))
	for i := range r {
		readers[i] = bufio.NewReader(r[i])
	}
	return &decoder{
		crc:          crc.New(0, crcTable),
		bufioReaders: readers,
	}
}

// decode decodes data in decoder and save in rec.
func (d *decoder) decode(rec *walpb.Record) error {
	rec.Reset()

	d.mu.Lock()
	err := d.decodeRecord(rec)
	d.mu.Unlock()

	return err
}

func (d *decoder) decodeRecord(rec *walpb.Record) error {
	if len(d.bufioReaders) == 0 {
		return io.EOF
	}

	lengthFieldN, err := readLengthFieldN(d.bufioReaders[0])

	switch {
	case err == io.EOF || (err == nil && lengthFieldN == 0):
		// end of file, or end of preallocated space
		d.bufioReaders = d.bufioReaders[1:]
		if len(d.bufioReaders) == 0 {
			return io.EOF
		}
		d.lastValidOffset = 0

		// decode next reader
		// if err == io.EOF && len(d.bufioReaders) > 0
		// OR
		// if err == nil && lengthFieldN == 0 && len(d.bufioReaders) > 0
		return d.decodeRecord(rec)

	case err != nil:
		return err
	}

	// continue if err == nil && lengthFieldN > 0

	dataN, padBytesN := decodeLengthFieldN(lengthFieldN)
	data := make([]byte, dataN+padBytesN)
	if _, err = io.ReadFull(d.bufioReaders[0], data); err != nil {
		// ReadFull returns io.EOF only when no bytes were read.
		// decoder should treat this as io.ErrUnexpectedEOF
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}

	// unsharshal, and check torn entry if error
	if err = rec.Unmarshal(data[:dataN:dataN]); err != nil {
		if d.isTornEntry(data) {
			return io.ErrUnexpectedEOF
		}
		return err
	}

	// skip CRC checking if the record is walpb.RECORD_TYPE_CRC
	if rec.Type != walpb.RECORD_TYPE_CRC {
		d.crc.Write(rec.Data)
		if err = rec.Validate(d.crc.Sum32()); err != nil { // double-check CRC
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

const minSectorSize = 512

// isTornEntry returns true when the last entry of the WAL
// was partially written and corrupted because of a torn write.
func (d *decoder) isTornEntry(data []byte) bool {
	if len(d.bufioReaders) != 1 { // not the last leader
		return false
	}

	// split data on sector boundaries
	const minSectorSize = 512
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
