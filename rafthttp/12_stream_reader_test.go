package rafthttp

import (
	"testing"

	"github.com/gyuho/db/pkg/types"
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

}

// (etcd rafthttp.TestStream)
func Test_streamReader(t *testing.T) {

}

// (etcd rafthttp.TestStreamReaderStopOnConnect)
func Test_streamReader_stop_on_connect(t *testing.T) {

}
