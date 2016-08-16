package rafthttp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"time"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/version"
)

// (etcd rafthttp.TestServeRaftStreamPrefix)
func Test_streamHandler(t *testing.T) {
	tests := []struct {
		path string
	}{
		{
			path.Join(PrefixRaftStreamMessage, "1"),
		},
	}
	for i, tt := range tests {
		req, err := http.NewRequest("GET", "http://localhost:2380"+tt.path, nil)
		if err != nil {
			t.Fatalf("#%d: could not create request: %v", i, err)
		}
		req.Header.Set(HeaderClusterID, "1")
		req.Header.Set(HeaderServerVersion, version.ServerVersion)
		req.Header.Set(HeaderToID, "2")

		peer := newFakePeer()
		peerGetter := &fakePeerGetter{peers: map[types.ID]Peer{types.ID(1): peer}}
		hd := newStreamHandler(&Transport{}, &fakeRaft{}, peerGetter, types.ID(2), types.ID(1))

		rw := httptest.NewRecorder()
		go hd.ServeHTTP(rw, req)

		var conn *outgoingConn
		select {
		case conn = <-peer.connc:
		case <-time.After(time.Second):
			t.Fatalf("#%d: failed to attach outgoingConn", i)
		}
		if g := rw.Header().Get(HeaderServerVersion); g != version.ServerVersion {
			t.Errorf("#%d: version expected %s, got %s, ", i, version.ServerVersion, g)
		}
		conn.Close()
	}
}

// (etcd rafthttp.TestServeRaftStreamPrefixBad)
func Test_streamHandler_bad(t *testing.T) {
	removedID := uint64(5)
	tests := []struct {
		method    string
		path      string
		clusterID string
		remote    string

		wcode int
	}{
		{ // bad method
			"PUT",
			PrefixRaftStream + "/message/1",
			"1",
			"1",
			http.StatusMethodNotAllowed,
		},
		{ // bad method
			"POST",
			PrefixRaftStream + "/message/1",
			"1",
			"1",
			http.StatusMethodNotAllowed,
		},
		{ // bad method
			"DELETE",
			PrefixRaftStream + "/message/1",
			"1",
			"1",
			http.StatusMethodNotAllowed,
		},
		{ // bad path
			"GET",
			PrefixRaftStream + "/strange/1",
			"1",
			"1",
			http.StatusNotFound,
		},
		{ // bad path
			"GET",
			PrefixRaftStream + "/strange",
			"1",
			"1",
			http.StatusNotFound,
		},
		{ // non-existent peer
			"GET",
			PrefixRaftStream + "/message/2",
			"1",
			"1",
			http.StatusNotFound,
		},
		{ // removed peer
			"GET",
			PrefixRaftStream + "/message/" + fmt.Sprint(removedID),
			"1",
			"1",
			http.StatusGone,
		},
		{ // wrong cluster ID
			"GET",
			PrefixRaftStream + "/message/1",
			"2",
			"1",
			http.StatusPreconditionFailed,
		},
		{ // wrong remote id
			"GET",
			PrefixRaftStream + "/message/1",
			"1",
			"2",
			http.StatusPreconditionFailed,
		},
	}
	for i, tt := range tests {
		req, err := http.NewRequest(tt.method, "http://localhost:2380"+tt.path, nil)
		if err != nil {
			t.Fatalf("#%d: could not create request: %v", i, err)
		}
		req.Header.Set(HeaderClusterID, tt.clusterID)
		req.Header.Set(HeaderServerVersion, version.ServerVersion)
		req.Header.Set(HeaderToID, tt.remote)

		rw := httptest.NewRecorder()

		peerGetter := &fakePeerGetter{peers: map[types.ID]Peer{types.ID(1): newFakePeer()}}
		hd := newStreamHandler(&Transport{}, &fakeRaft{removedID: removedID}, peerGetter, types.ID(1), types.ID(1))

		hd.ServeHTTP(rw, req)

		if rw.Code != tt.wcode {
			t.Fatalf("#%d: code expect %d, got %d", i, tt.wcode, rw.Code)
		}
	}
}
