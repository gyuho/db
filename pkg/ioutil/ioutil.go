package ioutil

import (
	"fmt"
	"io"
)

// (etcd ioutil.limitedBufferReader)
type limitedBufferReader struct {
	r           io.Reader
	totalBytesN int
}

func (r *limitedBufferReader) Read(p []byte) (n int, err error) {
	np := p
	if len(np) > r.totalBytesN {
		np = np[:r.totalBytesN:r.totalBytesN]
	}

	return r.r.Read(np)
}

// NewLimitedBufferReader returns io.Reader that reads from the given reader
// but returns at most n bytes.
//
// (etcd ioutil.NewLimitedBufferReader)
func NewLimitedBufferReader(r io.Reader, totalBytesN int) io.Reader {
	return &limitedBufferReader{
		r:           r,
		totalBytesN: totalBytesN,
	}
}

var (
	ErrShortRead = fmt.Errorf("ioutil: short read")
	ErrExpectEOF = fmt.Errorf("ioutil: expect EOF")
)

// NewExactReadCloser returns a ReadCloser of which Read returns errors
// if the underlying reader does not read back exactly the requested number
// of bytes.
//
// (etcd ioutil.NewExactReadCloser)
func NewExactReadCloser(rc io.ReadCloser, totalBytesN int64) io.ReadCloser {
	return &exactReadCloser{
		rc:          rc,
		totalBytesN: totalBytesN,
	}
}

// (etcd ioutil.exactReadCloser)
type exactReadCloser struct {
	rc          io.ReadCloser
	totalBytesN int64
	read        int64
}

func (e *exactReadCloser) Read(p []byte) (int, error) {
	n, err := e.rc.Read(p)
	e.read += int64(n)
	if e.read > e.totalBytesN {
		return 0, ErrExpectEOF
	}
	if e.read < e.totalBytesN && n == 0 {
		return 0, ErrShortRead
	}

	return n, err
}

func (e *exactReadCloser) Close() error {
	if err := e.rc.Close(); err != nil {
		return err
	}
	if e.read < e.totalBytesN {
		return ErrShortRead
	}
	return nil
}
