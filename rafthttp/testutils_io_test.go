package rafthttp

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
)

//////////////////////////////////////////////////////////////

// (etcd rafthttp.nopReadCloser)
type nopReaderCloser struct{}

func (n *nopReaderCloser) Read(p []byte) (int, error) { return 0, io.EOF }
func (n *nopReaderCloser) Close() error               { return nil }

//////////////////////////////////////////////////////////////

// (etcd rafthttp.strReaderCloser)
type stringReaderCloser struct{ *strings.Reader }

func (s stringReaderCloser) Close() error { return nil }

//////////////////////////////////////////////////////////////

// (etcd rafthttp.errReader)
type errorReader struct{}

func (er *errorReader) Read(_ []byte) (int, error) { return 0, errors.New("some error") }

//////////////////////////////////////////////////////////////

// (etcd rafthttp.errReadCloser)
type errorReaderCloser struct{ err error }

func (s *errorReaderCloser) Read(p []byte) (int, error) { return 0, s.err }
func (s *errorReaderCloser) Close() error               { return s.err }

//////////////////////////////////////////////////////////////

// (etcd rafthttp.waitReadCloser)
type waitReaderCloser struct{ closec chan struct{} }

// (etcd rafthttp.newWaitReadCloser)
func newWaitReaderCloser() *waitReaderCloser {
	return &waitReaderCloser{make(chan struct{})}
}

func (wrc *waitReaderCloser) Read(p []byte) (int, error) {
	<-wrc.closec
	return 0, io.EOF
}

func (wrc *waitReaderCloser) Close() error {
	close(wrc.closec)
	return nil
}

//////////////////////////////////////////////////////////////

// fakeWriterToResponse implements writerToResponse.
//
// (etcd rafthttp.resWriterToError)
type fakeWriterToResponse struct {
	code int
}

func (e *fakeWriterToResponse) Error() string                  { return "" }
func (e *fakeWriterToResponse) WriteTo(rw http.ResponseWriter) { rw.WriteHeader(e.code) }

//////////////////////////////////////////////////////////////

// (etcd rafthttp.fakeWriteFlushCloser)
type fakeWriterFlusherCloser struct {
	mu      sync.Mutex
	err     error
	written int
	closed  chan struct{}
	writec  chan struct{}
}

func newFakeWriterFlusherCloser(err error) *fakeWriterFlusherCloser {
	return &fakeWriterFlusherCloser{
		err:    err,
		closed: make(chan struct{}),
		writec: make(chan struct{}, 1),
	}
}

func (wfc *fakeWriterFlusherCloser) Write(p []byte) (n int, err error) {
	wfc.mu.Lock()
	defer wfc.mu.Unlock()
	select {
	case wfc.writec <- struct{}{}:
	default:
	}
	wfc.written += len(p)
	return len(p), wfc.err
}

func (wfc *fakeWriterFlusherCloser) Flush() {}

func (wfc *fakeWriterFlusherCloser) Close() error {
	close(wfc.closed)
	return wfc.err
}

func (wfc *fakeWriterFlusherCloser) getClosed() bool {
	select {
	case <-wfc.closed:
		return true
	default:
		return false
	}
}

//////////////////////////////////////////////////////////////
