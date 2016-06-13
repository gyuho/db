package wal

import (
	"bufio"
	"hash"
	"hash/crc32"
	"io"
	"sync"

	"github.com/gyuho/distdb/crc"
	"github.com/gyuho/distdb/walpb"
)

var (
	crcTable = crc32.MakeTable(crc32.Castagnoli)
)

type encoder struct {
	mu sync.Mutex

	crc hash.Hash32

	// recordBuf pre-allocates bytes for walpb.Record Marshal.
	// etcd pre-allocates gogo/protobuf/proto.MarshalTo(make([]byte, 1024 * 1024))
	recordBuf []byte
	wordBuf   []byte

	bufioWriter *bufio.Writer
}

func newEncoder(w io.Writer, prevCRC uint32) *encoder {
	return &encoder{
		crc: crc.New(prevCRC, crcTable),

		// 1MB buffer
		recordBuf: make([]byte, 1024*1024),
		wordBuf:   make([]byte, byteBitN),

		bufioWriter: bufio.NewWriter(w),
	}
}

func (e *encoder) encode(rec *walpb.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// update the hash with new data
	e.crc.Write(rec.Data)
	rec.CRC = e.crc.Sum32()

	// marshal the Protocol Buffer
	var (
		data  []byte
		err   error
		dataN int

		recordBufN = len(e.recordBuf)
	)
	if rec.Size() > recordBufN { // not enough buffer was allocated
		// github.com/golang/protobuf/proto
		//
		// data, err = proto.Marshal(rec)
		//
		data, err = rec.Marshal()
		if err != nil {
			return err
		}
		dataN = len(data)

	} else { // use buffer
		// github.com/golang/protobuf/proto
		//
		// dataN = proto.Size(rec)
		// buf := proto.NewBuffer(e.recordBuf)
		// if err = buf.Marshal(rec); err != nil {
		// 	return err
		// }
		// data = buf.Bytes()[:dataN]
		//
		dataN, err = rec.MarshalTo(e.recordBuf)
		if err != nil {
			return err
		}
		data = e.recordBuf[:dataN]
	}

	// compute header and padding
	padBytesN := dataNToPadBytesN(dataN)
	lengthFieldN := encodeLengthFieldN(dataN, padBytesN)

	// write header
	if err = putLengthFieldN(e.bufioWriter, e.wordBuf, lengthFieldN); err != nil {
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
