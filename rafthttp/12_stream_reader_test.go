package rafthttp

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gyuho/db/pkg/testutil"
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/version"
)

// (etcd rafthttp.TestStreamReaderDialRequest)
func Test_streamReader_dial_request(t *testing.T) {
	tr := &roundTripperRecorder{}
	sr := &streamReader{
		peerID: types.ID(2),
		picker: newURLPicker(types.MustNewURLs([]string{"http://localhost:2380"})),
		pt:     &PeerTransport{From: types.ID(1), ClusterID: types.ID(1), streamRoundTripper: tr},
	}
	_, err := sr.dial()
	if err == nil {
		t.Fatalf("expected error, got %v", err)
	}

	req := tr.Request()
	wurl := "http://localhost:2380" + PrefixRaftStreamMessage + "/1"
	if req.URL.String() != wurl {
		t.Fatalf("URL expected %s, got %s", wurl, req.URL.String())
	}
	if w := "GET"; req.Method != w {
		t.Fatalf("method expected %s, got %s", w, req.Method)
	}
	if g := req.Header.Get(HeaderClusterID); g != "1" {
		t.Fatalf("header HeaderClusterID expected 1, got %s", g)
	}
	if g := req.Header.Get(HeaderToID); g != "2" {
		t.Fatalf("header HeaderToID expected 2, got %s", g)
	}
}

// (etcd rafthttp.TestStreamReaderDialResult)
func Test_streamReader_dial_result(t *testing.T) {
	tests := []struct {
		code  int
		err   error
		wok   bool
		whalt bool
	}{
		{0, errors.New("blah"), false, false},
		{http.StatusOK, nil, true, false},
		{http.StatusMethodNotAllowed, nil, false, false},
		{http.StatusNotFound, nil, false, false},
		{http.StatusPreconditionFailed, nil, false, false},
		{http.StatusGone, nil, false, true},
	}

	for i, tt := range tests {
		h := http.Header{}
		h.Add(HeaderServerVersion, version.ServerVersion)

		tr := &respRoundTripper{
			code:   tt.code,
			header: h,
			err:    tt.err,
		}

		sr := &streamReader{
			peerID: types.ID(2),
			picker: newURLPicker(types.MustNewURLs([]string{"http://localhost:2380"})),
			pt:     &PeerTransport{ClusterID: types.ID(1), streamRoundTripper: tr},
			errc:   make(chan error, 1),
		}

		_, err := sr.dial()
		if ok := err == nil; ok != tt.wok {
			t.Fatalf("#%d: ok = %v, want %v", i, ok, tt.wok)
		}
		if halt := len(sr.errc) > 0; halt != tt.whalt {
			t.Fatalf("#%d: halt = %v, want %v", i, halt, tt.whalt)
		}
	}
}

// (etcd rafthttp.TestStreamReaderStopOnConnect)
func Test_streamReader_stop_on_connect(t *testing.T) {
	defer testutil.AfterTest(t)
	h := http.Header{}
	h.Add(HeaderServerVersion, version.ServerVersion)

	tr := &respRoundTripperWait{rt: &respRoundTripper{code: http.StatusOK, header: h}}
	sr := &streamReader{
		peerID: types.ID(2),
		status: newPeerStatus(types.ID(2)),
		picker: newURLPicker(types.MustNewURLs([]string{"http://localhost:2380"})),
		pt:     &PeerTransport{ClusterID: types.ID(1), streamRoundTripper: tr},
		errc:   make(chan error, 1),
	}
	tr.onResp = func() {
		go sr.stop()
		time.Sleep(10 * time.Millisecond)
	}

	sr.start()

	select {
	case <-sr.donec:
	case <-time.After(time.Second):
		t.Fatal("did not close in time")
	}
}

// (etcd rafthttp.TestStream)
func Test_streamReader(t *testing.T) {
}
