package rafthttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/gyuho/db/pkg/scheduleutil"
	"github.com/gyuho/db/pkg/testutil"
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/version"
)

// (etcd rafthttp.TestStreamReaderDialRequest)
func Test_streamReader_dial_request(t *testing.T) {
	tr := &roundTripperRecorder{rec: scheduleutil.NewRecorderBuffered()}
	sr := &streamReader{
		peerID:    types.ID(2),
		picker:    newURLPicker(types.MustNewURLs([]string{"http://localhost:2380"})),
		transport: &Transport{Sender: types.ID(1), ClusterID: types.ID(1), streamRoundTripper: tr},
	}
	_, err := sr.dial()
	if err == nil {
		t.Fatalf("expected error, got %v", err)
	}

	act, err := tr.rec.Wait(1)
	if err != nil {
		t.Fatal(err)
	}
	req := act[0].Parameters[0].(*http.Request)

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
			peerID:    types.ID(2),
			picker:    newURLPicker(types.MustNewURLs([]string{"http://localhost:2380"})),
			transport: &Transport{ClusterID: types.ID(1), streamRoundTripper: tr},
			errc:      make(chan error, 1),
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
		peerID:    types.ID(2),
		status:    newPeerStatus(types.ID(2)),
		picker:    newURLPicker(types.MustNewURLs([]string{"http://localhost:2380"})),
		transport: &Transport{ClusterID: types.ID(1), streamRoundTripper: tr},
		errc:      make(chan error, 1),
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
func Test_streamReader_streamWriter(t *testing.T) {
	recvc := make(chan raftpb.Message, streamBufferN)
	propc := make(chan raftpb.Message, streamBufferN)
	msgapp := raftpb.Message{
		Type:              raftpb.MESSAGE_TYPE_LEADER_APPEND,
		From:              2,
		To:                1,
		SenderCurrentTerm: 1,
		LogIndex:          3,
		LogTerm:           1,
		Entries:           []raftpb.Entry{{Index: 4, Term: 1}},
	}

	tests := []struct {
		m       raftpb.Message
		msgChan chan raftpb.Message
	}{
		{
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, To: 2},
			propc,
		},
		{
			msgapp,
			recvc,
		},
	}

	for i, tt := range tests {
		h := &fakeStreamHandler{}
		srv := httptest.NewServer(h)
		defer srv.Close()

		sw := startStreamWriter(types.ID(1), newPeerStatus(types.ID(1)), &fakeRaft{})
		defer sw.stop()

		h.sw = sw

		picker := newURLPicker(types.MustNewURLs([]string{srv.URL}))

		sr := &streamReader{
			peerID: types.ID(2),
			status: newPeerStatus(types.ID(2)),

			picker:    picker,
			transport: &Transport{ClusterID: types.ID(1), streamRoundTripper: &http.Transport{}},

			recvc:         recvc,
			propc: propc,
		}

		sr.start()

		// wait for stream to work
		var writec chan<- raftpb.Message
		for {
			var ok bool
			if writec, ok = sw.messageChanToSend(); ok {
				break
			}
			time.Sleep(time.Millisecond)
		}

		writec <- tt.m
		var m raftpb.Message
		select {
		case m = <-tt.msgChan:
		case <-time.After(time.Second):
			t.Fatalf("#%d: failed to receive message from the channel", i)
		}
		if !reflect.DeepEqual(m, tt.m) {
			t.Fatalf("#%d: message expected %+v, got = %+v", i, tt.m, m)
		}

		sr.stop()
	}
}
