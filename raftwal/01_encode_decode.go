package raftwal

import (
	"bufio"
	"encoding/binary"
	"hash"
	"hash/crc32"
	"io"
	"os"
	"sync"

	"github.com/gyuho/db/pkg/crcutil"
	"github.com/gyuho/db/pkg/ioutil"
	"github.com/gyuho/db/raftwal/raftwalpb"
)

// split data on sector boundaries
//
// (etcd wal.minSectorSize)
const minSectorSize = 512

// walPageBytes is the alignment for flushing records to the backing Writer.
// It should be a multiple of the minimum sector size so that WAL can be repaired
// safely between torn writes and ordinary data corruption.
//
// (etcd wal.walPageBytes)
const walPageBytes = 8 * minSectorSize

const (
	systemBitN = 64 // 64-bit system
	byteBitN   = 8  // 8-bit == 1-byte

	// alignedBitN is the number of bits to align into the data.
	//
	// 64-bit processor has 64 bits word size.
	// Word is the smallest piece of data that the processor can fetch from memory.
	//
	//
	// Computer reads from or writes to a memory address in word-sized chunks
	// or larger. Data alignment is to put this data to a memory address with
	// the size of "multiple" word-sized chunks. This increases the sytem
	// performance because that's how CPU handles the memory.
	//
	// To align the data this way, it needs to insert some meaningless bytes between
	// multiple data, which is data structure padding.
	//
	// etcd uses padding to prevent torn writes.
	// Each record is 8-byte aligned, so that the length field is never torn.
	//
	// 64-bit aligned data structure is called 64/8 byte(8-byte) aligned.
	// https://en.wikipedia.org/wiki/Data_structure_alignment
	alignedBitN = systemBitN / byteBitN

	lowerBitN = systemBitN - alignedBitN // lower 56-bit
)

/*
& (AND)

Let f be &

	1. f(a, b) = f(b, a)
	2. f(a, a) = a
	3. f(a, b) ≤ max(a, b)


∨ (OR)

Let f be ∨

	1. f(a, b) = f(b, a)
	2. f(a, a) = a
	3. f(a, b) ≥ max(a, b)
*/

// https://en.wikipedia.org/wiki/Data_structure_alignment
func getPadBytesN(dataN int) int {
	return (alignedBitN - (dataN % alignedBitN)) % alignedBitN
}

func encodeLen(dataN, padBytesN int) (headerN uint64) {
	headerN = uint64(dataN)
	if padBytesN != 0 {
		// 0x80 is 128 in decimal, 10000000 in binary
		// 1. encode pad bytes with sign bit (10000000)
		// 2. left-shift to lower 56-bit
		headerN = headerN | uint64(0x80|padBytesN)<<lowerBitN
	}
	return
}

func decodeLen(headerN int64) (dataN int64, padBytesN int64) {
	// revert left-shift from encoding
	// 0xFF is 11111111 in binary
	shift := uint64(0xFF) << lowerBitN
	dataN = int64(uint64(headerN) &^ shift)

	if headerN < 0 { // padding was encoded in lower 3-bit of length MSB
		// Since the max pad-bit is 7 (out of 8),
		// we can use 0x7 to undo bit shifts.
		// 0x7 is 111 in binary
		revt := uint64(headerN) >> lowerBitN
		padBytesN = int64(revt & 0x7)
	}
	return
}

func writeHeaderN(w io.Writer, buf []byte, headerN uint64) error {
	binary.LittleEndian.PutUint64(buf, headerN)
	_, err := w.Write(buf)
	return err
}

func readHeaderN(r io.Reader) (headerN int64, err error) {
	// Read with signed int64 in order to check the sign bit
	// which tells if pad bytes were encoded in lower bits or not.
	//
	// error is EOF only if no bytes were read.
	// If an EOF happens after reading some but not all the bytes,
	// binary.Read returns io.ErrUnexpectedEOF.
	err = binary.Read(r, binary.LittleEndian, &headerN)
	return
}

type encoder struct {
	mu        sync.Mutex
	crc       hash.Hash32
	recordBuf []byte
	wordBuf   []byte

	// bw *bufio.Writer
	bw *ioutil.PageWriter
}

var crcTable = crc32.MakeTable(crc32.Castagnoli)

const recordBufN = 1024 * 1024 // 1 MB buffer

func newEncoder(w io.Writer, prevCRC uint32, pageOffset int) *encoder {
	return &encoder{
		crc: crcutil.New(prevCRC, crcTable),

		recordBuf: make([]byte, recordBufN),
		wordBuf:   make([]byte, byteBitN),

		// bw: bufio.NewWriter(w),
		bw: ioutil.NewPageWriter(w, walPageBytes, pageOffset),
	}
}

// newFileEncoder creates a new encoder with current file offset for the page writer.
func newFileEncoder(f *os.File, prevCrc uint32) (*encoder, error) {
	offset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	return newEncoder(f, prevCrc, int(offset)), nil
}

func (e *encoder) encode(rec *raftwalpb.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.crc.Write(rec.Data)
	rec.CRC = e.crc.Sum32()

	var (
		data  []byte
		err   error
		dataN int

		recordBufN = len(e.recordBuf)
	)
	if rec.Size() > recordBufN { // not enough buffer was allocated
		data, err = rec.Marshal()
		if err != nil {
			return err
		}
		dataN = len(data)
	} else {
		dataN, err = rec.MarshalTo(e.recordBuf)
		if err != nil {
			return err
		}
		data = e.recordBuf[:dataN]
	}

	padBytesN := getPadBytesN(dataN)
	headerN := encodeLen(dataN, padBytesN)
	if err = writeHeaderN(e.bw, e.wordBuf, headerN); err != nil {
		return err
	}

	// write data + padding
	if padBytesN != 0 {
		data = append(data, make([]byte, padBytesN)...)
	}
	_, err = e.bw.Write(data)
	return err
}

func (e *encoder) flush() error {
	e.mu.Lock()
	err := e.bw.Flush()
	e.mu.Unlock()
	return err
}

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
		fileOff = d.lastValidOffset + byteBitN
		curOff  int

		chunks [][]byte
	)
	for curOff < len(data) {
		chunkN := int(minSectorSize - (fileOff % minSectorSize))
		if chunkN > len(data)-curOff { // to prevent index out of bound
			chunkN = len(data) - curOff
		}

		endIdx := curOff + chunkN
		chunks = append(chunks, data[curOff:endIdx])

		fileOff += int64(chunkN)
		curOff += chunkN
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
