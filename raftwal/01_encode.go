package raftwal

import (
	"bufio"
	"encoding/binary"
	"hash"
	"hash/crc32"
	"io"
	"sync"

	"github.com/gyuho/db/pkg/crcutil"
	"github.com/gyuho/db/raftwal/raftwalpb"
)

const (
	systemBitN = 64 // 64-bit system
	byteBitN   = 8  // 1-byte is 8-bit

	// wordBitN is the size of a word chunk. Computer reads from or
	// writes to a memory address in word-sized chunks or larger.
	wordBitN = systemBitN / byteBitN

	// alignedBitN is the number of bits to align into the data.
	//
	// 64-bit aligned data structure is called 64/8 byte(8-byte) aligned.
	//
	// Computer reads from or writes to a memory address in word-sized chunks
	// or larger. Data alignment is to put this data to a memory address with
	// the size of multiple word-sized chunks. This increases the sytem
	// performance because that's how CPU handles the memory. To align the
	// data this way, it needs to insert some meaningless bytes between
	// multiple data, which is data structure padding.
	//
	// etcd uses padding to prevent torn writes.
	// Each record is 8-byte aligned, so that the length field is never torn.
	alignedBitN = wordBitN

	lowerBitN = systemBitN - alignedBitN // lower 56-bit
)

// https://en.wikipedia.org/wiki/Data_structure_alignment
func getPadBytesN(dataN int) int {
	return (alignedBitN - (dataN % alignedBitN)) % alignedBitN
}

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
	// 0xff is 11111111 in binary
	shift := uint64(0xff) << lowerBitN
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
	mu          sync.Mutex
	crc         hash.Hash32
	recordBuf   []byte
	wordBuf     []byte
	bufioWriter *bufio.Writer
}

var crcTable = crc32.MakeTable(crc32.Castagnoli)

func newEncoder(w io.Writer, prevCRC uint32) *encoder {
	return &encoder{
		crc: crcutil.New(prevCRC, crcTable),

		recordBuf: make([]byte, 1024*1024), // 1MB buffer
		wordBuf:   make([]byte, byteBitN),

		bufioWriter: bufio.NewWriter(w),
	}
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
	if err = writeHeaderN(e.bufioWriter, e.wordBuf, headerN); err != nil {
		return err
	}

	// write data + padding
	if padBytesN != 0 {
		data = append(data, make([]byte, padBytesN)...)
	}
	_, err = e.bufioWriter.Write(data)
	return err
}

func (e *encoder) flush() error {
	e.mu.Lock()
	err := e.bufioWriter.Flush()
	e.mu.Unlock()
	return err
}
