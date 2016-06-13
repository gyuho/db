package wal

import (
	"encoding/binary"
	"io"
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
	alignedBitN = wordBitN

	lowerBitN = systemBitN - alignedBitN
)

// dataNToPadBytesN computes the number of bytes required for data padding.
// https://en.wikipedia.org/wiki/Data_structure_alignment
func dataNToPadBytesN(dataN int) int {
	return (alignedBitN - (dataN % alignedBitN)) % alignedBitN
}

// encodeLengthFieldN computes the length field and number of padding bytes
// required to align the data.
//
// A WAL record is composed of:
//
//   - length field: 64-bit(8-byte) packed data structure to store
//                   the record size, in lower 56-bit of 64-bit length
//
//   - WAL record protobuf
//      - CRC: CRC32 value of all record protobufs preceding the current record
//      - data payload(bytes)
//
//   - padding, if needed
//
// Each record is 8-byte aligned, so that the length field is never torn.
//
func encodeLengthFieldN(dataN, padBytesN int) (lengthFieldN uint64) {
	lengthFieldN = uint64(dataN)
	if padBytesN != 0 {
		// 0x80 is 128 in decimal, 10000000 in binary
		// 1. encode pad bytes with sign bit (10000000)
		// 2. left-shift to lower 56-bit
		lengthFieldN = lengthFieldN | uint64(0x80|padBytesN)<<lowerBitN
	}
	return
}

func decodeLengthFieldN(lengthFieldN int64) (dataN int64, padBytesN int64) {
	// undo the bit-shifts from previous encoding
	// 0xff is 11111111 in binary
	shift := uint64(0xff) << lowerBitN
	dataN = int64(uint64(lengthFieldN) &^ shift)

	if lengthFieldN < 0 { // padding was encoded in lower 3-bit of length MSB
		// Since the max pad-bit is 7 (out of 8),
		// we can use 0x7 to undo bit shifts.
		// 0x7 is 111 in binary
		revt := uint64(lengthFieldN) >> lowerBitN
		padBytesN = int64(revt & 0x7)
	}

	return
}

func putLengthFieldN(w io.Writer, buf []byte, lengthFieldN uint64) error {
	binary.LittleEndian.PutUint64(buf, lengthFieldN)
	_, err := w.Write(buf)
	return err
}

// readLengthFieldN reads lengthFieldN from the reader.
// Read with signed int64 in order to check the sign bit
// which tells if pad bytes were encoded in lower bits or not.
func readLengthFieldN(r io.Reader) (lengthFieldN int64, err error) {
	// error is EOF only if no bytes were read.
	// If an EOF happens after reading some but not all the bytes,
	// Read returns ErrUnexpectedEOF.
	err = binary.Read(r, binary.LittleEndian, &lengthFieldN)
	return
}
