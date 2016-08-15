package rafthttp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"

	"github.com/gyuho/db/pkg/scheduleutil"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/version"
)

// (etcd rafthttp.fakeRaft)
type fakeRaft struct {
	recvc     chan<- raftpb.Message
	removedID uint64
	err       error
}

func (p *fakeRaft) Process(ctx context.Context, m raftpb.Message) error {
	select {
	case p.recvc <- m:
	default:
	}
	return p.err
}

func (p *fakeRaft) IsIDRemoved(id uint64) bool { return id == p.removedID }

func (p *fakeRaft) ReportUnreachable(id uint64) {}

func (p *fakeRaft) ReportSnapshot(id uint64, status raftpb.SNAPSHOT_STATUS) {}

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

// (etcd rafthttp.fakeStreamHandler)
type fakeStreamHandler struct {
	sw *streamWriter
}

func (h *fakeStreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add(HeaderServerVersion, version.ServerVersion)
	w.(http.Flusher).Flush()
	c := newCloseNotifier()
	h.sw.attachOutgoingConn(&outgoingConn{
		Writer:  w,
		Flusher: w.(http.Flusher),
		Closer:  c,
	})
	<-c.closeNotify()
}

// (etcd rafthttp.nopReadCloser)
type nopReadCloser struct{}

func (n *nopReadCloser) Read(p []byte) (int, error) { return 0, io.EOF }
func (n *nopReadCloser) Close() error               { return nil }

// (etcd rafthttp.roundTripperRecorder)
type roundTripperRecorder struct {
	mu  sync.Mutex
	req *http.Request
}

func (t *roundTripperRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.req = req
	return &http.Response{StatusCode: http.StatusNoContent, Body: &nopReadCloser{}}, nil
}

func (t *roundTripperRecorder) Request() *http.Request {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.req
}

// (etcd rafthttp.roundTripperBlocker)
type roundTripperBlocker struct {
	unblockc chan struct{}

	mu     sync.Mutex
	cancel map[*http.Request]chan struct{}
}

// (etcd rafthttp.newRoundTripperBlocker)
func newRoundTripperBlocker() *roundTripperBlocker {
	return &roundTripperBlocker{
		unblockc: make(chan struct{}),
		cancel:   make(map[*http.Request]chan struct{}),
	}
}

func (t *roundTripperBlocker) RoundTrip(req *http.Request) (*http.Response, error) {
	c := make(chan struct{}, 1)

	t.mu.Lock()
	t.cancel[req] = c
	t.mu.Unlock()

	select {
	case <-t.unblockc:
		return &http.Response{StatusCode: http.StatusNoContent, Body: &nopReadCloser{}}, nil

	case <-c:
		return nil, errors.New("request canceled")

	case <-req.Context().Done():
		return nil, errors.New("request canceled")

		// DEPRECATED:
		// case <-req.Cancel:
		// return nil, errors.New("request canceled")
	}
}

func (t *roundTripperBlocker) unblock() {
	close(t.unblockc)
}

func (t *roundTripperBlocker) CancelRequest(req *http.Request) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if c, ok := t.cancel[req]; ok {
		c <- struct{}{}
		delete(t.cancel, req)
	}
}

// (etcd rafthttp.respRoundTripper)
type respRoundTripper struct {
	mu  sync.Mutex
	rec scheduleutil.Recorder

	code   int
	header http.Header
	err    error
}

// (etcd rafthttp.newRespRoundTripper)
func newRespRoundTripper(code int, err error) *respRoundTripper {
	return &respRoundTripper{code: code, err: err}
}

func (t *respRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.rec != nil {
		t.rec.Record(scheduleutil.Action{Name: "req", Parameters: []interface{}{req}})
	}
	return &http.Response{StatusCode: t.code, Header: t.header, Body: &nopReadCloser{}}, t.err
}

// (etcd rafthttp.respWaitRoundTripper)
type respRoundTripperWait struct {
	rt     *respRoundTripper
	onResp func()
}

func (t *respRoundTripperWait) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.rt.RoundTrip(req)
	resp.Body = newWaitReadCloser()
	t.onResp()
	return resp, err
}

// (etcd rafthttp.waitReadCloser)
type waitReadCloser struct{ closec chan struct{} }

// (etcd rafthttp.newWaitReadCloser)
func newWaitReadCloser() *waitReadCloser {
	return &waitReadCloser{make(chan struct{})}
}

func (wrc *waitReadCloser) Read(p []byte) (int, error) {
	<-wrc.closec
	return 0, io.EOF
}

func (wrc *waitReadCloser) Close() error {
	close(wrc.closec)
	return nil
}
