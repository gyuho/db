package ioutil

import (
	"bytes"
	"io"
	"testing"
)

func TestLimitedBufferReaderRead(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 100))
	limit := 10
	lr := NewLimitedBufferReader(buf, limit)

	n, err := lr.Read(make([]byte, 100))
	if err != nil {
		t.Fatal(err)
	}
	if n != limit {
		t.Fatalf("expected %d, got %d", limit, n)
	}
}

type readerNilCloser struct {
	io.Reader
}

func (rc *readerNilCloser) Close() error {
	return nil
}

// TestExactReadCloserExpectEOF expects an ErrExpectEOF when reading too much.
func TestExactReadCloserExpectEOF(t *testing.T) {
	var (
		buf               = bytes.NewBuffer(make([]byte, 10))
		totalBytesN int64 = 1
	)
	rc := NewExactReadCloser(&readerNilCloser{buf}, totalBytesN)
	if _, err := rc.Read(make([]byte, 10)); err != ErrExpectEOF {
		t.Fatalf("expected %v, got %v", ErrExpectEOF, err)
	}
}

// TestExactReadCloserExpectShort expects an ErrShortRead when reading too much.
func TestExactReadCloserExpectShort(t *testing.T) {
	var (
		buf               = bytes.NewBuffer(make([]byte, 5))
		totalBytesN int64 = 10
	)
	rc := NewExactReadCloser(&readerNilCloser{buf}, totalBytesN)
	if _, err := rc.Read(make([]byte, 10)); err != nil {
		t.Fatal(err)
	}
	if err := rc.Close(); err != ErrShortRead {
		t.Fatalf("expected %v, got %v", ErrShortRead, err)
	}
}
