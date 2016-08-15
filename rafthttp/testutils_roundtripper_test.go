package rafthttp

import (
	"errors"
	"net/http"
	"sync"

	"github.com/gyuho/db/pkg/scheduleutil"
)

//////////////////////////////////////////////////////////////

// (etcd rafthttp.roundTripperRecorder)
type roundTripperRecorder struct {
	mu  sync.Mutex
	req *http.Request
}

func (t *roundTripperRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.req = req
	return &http.Response{StatusCode: http.StatusNoContent, Body: &nopReaderCloser{}}, nil
}

func (t *roundTripperRecorder) Request() *http.Request {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.req
}

//////////////////////////////////////////////////////////////

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
	return &http.Response{StatusCode: t.code, Header: t.header, Body: &nopReaderCloser{}}, t.err
}

//////////////////////////////////////////////////////////////

// (etcd rafthttp.respWaitRoundTripper)
type respRoundTripperWait struct {
	rt     *respRoundTripper
	onResp func()
}

func (t *respRoundTripperWait) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.rt.RoundTrip(req)
	resp.Body = newWaitReaderCloser()
	t.onResp()
	return resp, err
}

//////////////////////////////////////////////////////////////

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
		return &http.Response{StatusCode: http.StatusNoContent, Body: &nopReaderCloser{}}, nil

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

//////////////////////////////////////////////////////////////
