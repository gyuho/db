package rafthttp

import (
	"io"
	"strings"
	"sync"
)

//////////////////////////////////////////////////////////////

// (etcd rafthttp.strReaderCloser)
type stringReaderCloser struct{ *strings.Reader }

func (s stringReaderCloser) Close() error { return nil }

//////////////////////////////////////////////////////////////

// (etcd rafthttp.errReadCloser)
type errorReaderCloser struct{ err error }

func (s *errorReaderCloser) Read(p []byte) (int, error) { return 0, s.err }
func (s *errorReaderCloser) Close() error               { return s.err }

//////////////////////////////////////////////////////////////

// (etcd rafthttp.fakeWriteFlushCloser)
type fakeWriterFlusherCloser struct {
	mu      sync.Mutex
	written int
	closed  bool
	err     error
}

func (wfc *fakeWriterFlusherCloser) Write(p []byte) (n int, err error) {
	wfc.mu.Lock()
	defer wfc.mu.Unlock()

	wfc.written += len(p)
	return len(p), wfc.err
}

func (wfc *fakeWriterFlusherCloser) Flush() {}

func (wfc *fakeWriterFlusherCloser) Close() error {
	wfc.mu.Lock()
	defer wfc.mu.Unlock()

	wfc.closed = true
	return wfc.err
}

func (wfc *fakeWriterFlusherCloser) getWritten() int {
	wfc.mu.Lock()
	defer wfc.mu.Unlock()

	return wfc.written
}

func (wfc *fakeWriterFlusherCloser) getClosed() bool {
	wfc.mu.Lock()
	defer wfc.mu.Unlock()

	return wfc.closed
}

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

// (etcd rafthttp.nopReadCloser)
type nopReaderCloser struct{}

func (n *nopReaderCloser) Read(p []byte) (int, error) { return 0, io.EOF }
func (n *nopReaderCloser) Close() error               { return nil }

//////////////////////////////////////////////////////////////
