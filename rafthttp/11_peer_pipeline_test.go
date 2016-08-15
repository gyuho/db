package rafthttp

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/coreos/etcd/pkg/testutil"
	"github.com/gyuho/db/pkg/scheduleutil"
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/version"
)

func startTestPeerPipeline(pt *PeerTransport, picker *urlPicker) *peerPipeline {
	p := &peerPipeline{
		peerID: types.ID(1),
		status: newPeerStatus(types.ID(1)),

		r: &fakeRaft{},

		picker: picker,
		pt:     pt,

		errc: make(chan error, 1),
	}
	p.start()
	return p
}

// (etcd rafthttp.TestPipelineSend)
func Test_peerPipeline_start(t *testing.T) {
	tr := &roundTripperRecorder{}
	pt := &PeerTransport{peerPipelineRoundTripper: tr}

	picker := newURLPicker(types.MustNewURLs([]string{"http://localhost:2380"}))
	pn := startTestPeerPipeline(pt, picker)

	pn.raftMessageChan <- raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND}

	scheduleutil.WaitSchedule()

	pn.stop()

	if tr.Request() == nil {
		t.Fatal("sender fails to post the data")
	}
}

// (etcd rafthttp.TestPipelineKeepSendingWhenPostError)
func Test_peerPipeline_send_error(t *testing.T) {
	tr := &respRoundTripper{rec: scheduleutil.NewRecorderStream(), err: fmt.Errorf("roundtrip error")}
	pt := &PeerTransport{peerPipelineRoundTripper: tr}

	picker := newURLPicker(types.MustNewURLs([]string{"http://localhost:2380"}))
	pn := startTestPeerPipeline(pt, picker)
	defer pn.stop()

	for i := 0; i < 50; i++ {
		pn.raftMessageChan <- raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND}
	}

	_, err := tr.rec.Wait(50)
	if err != nil {
		t.Fatal(err)
	}
}

// (etcd rafthttp.TestPipelineExceedMaximumServing)
func Test_peerPipeline_send_maximum(t *testing.T) {
	tr := newRoundTripperBlocker()
	pt := &PeerTransport{peerPipelineRoundTripper: tr}

	picker := newURLPicker(types.MustNewURLs([]string{"http://localhost:2380"}))
	pn := startTestPeerPipeline(pt, picker)
	defer pn.stop()

	scheduleutil.WaitSchedule()

	for i := 0; i < connPerPipeline+peerPipelineBufferN; i++ {
		select {
		case pn.raftMessageChan <- raftpb.Message{}:
		default:
			t.Fatal("failed to send out message")
		}

		// force the sender to grab data
		testutil.WaitSchedule()
	}

	// try to send a data when we are sure the buffer is full
	select {
	case pn.raftMessageChan <- raftpb.Message{}:
		t.Fatal("unexpected message sendout")
	default:
	}

	tr.unblock()

	scheduleutil.WaitSchedule()

	// It could send new data after previous ones succeed
	select {
	case pn.raftMessageChan <- raftpb.Message{}:
	default:
		t.Fatal("failed to send out message")
	}
}

// (etcd rafthttp.TestPipelinePost)
func Test_peerPipeline_send_post(t *testing.T) {
	tr := &roundTripperRecorder{}
	pt := &PeerTransport{ClusterID: types.ID(1), peerPipelineRoundTripper: tr}

	picker := newURLPicker(types.MustNewURLs([]string{"http://localhost:2380"}))
	pn := startTestPeerPipeline(pt, picker)
	if err := pn.post([]byte("testdata")); err != nil {
		t.Fatal(err)
	}
	pn.stop()

	if g := tr.Request().Method; g != "POST" {
		t.Fatalf("method = %s, want %s", g, "POST")
	}
	if g := tr.Request().URL.String(); g != "http://localhost:2380/raft" {
		t.Fatalf("url = %s, want %s", g, "http://localhost:2380/raft")
	}
	if g := tr.Request().Header.Get(HeaderContentType); g != HeaderContentProtobuf {
		t.Fatalf("content type = %s, want %s", g, HeaderContentProtobuf)
	}
	if g := tr.Request().Header.Get(HeaderServerVersion); g != version.ServerVersion {
		t.Fatalf("version = %s, want %s", g, version.ServerVersion)
	}
	if g := tr.Request().Header.Get(HeaderClusterID); g != "1" {
		t.Fatalf("cluster id = %s, want %s", g, "1")
	}
	b, err := ioutil.ReadAll(tr.Request().Body)
	if err != nil {
		t.Fatalf("unexpected ReadAll error: %v", err)
	}
	if string(b) != "testdata" {
		t.Fatalf("body = %s, want %s", b, "testdata")
	}
}

// (etcd rafthttp.TestPipelinePostBad)
func Test_peerPipeline_send_post_bad(t *testing.T) {
	tests := []struct {
		u    string
		code int
		err  error
	}{
		{"http://localhost:2380", 0, errors.New("testerror")},
		{"http://localhost:2380", http.StatusOK, nil},
		{"http://localhost:2380", http.StatusCreated, nil},
	}
	for i, tt := range tests {
		pt := &PeerTransport{peerPipelineRoundTripper: newRespRoundTripper(tt.code, tt.err)}
		picker := newURLPicker(types.MustNewURLs([]string{tt.u}))
		pn := startTestPeerPipeline(pt, picker)

		err := pn.post([]byte("testdata"))
		pn.stop()

		if err == nil {
			t.Fatalf("#%d: err = nil, want not nil", i)
		}
	}
}

// (etcd rafthttp.TestPipelinePostErrorc)
func Test_peerPipeline_send_post_error(t *testing.T) {
	tests := []struct {
		u    string
		code int
		err  error
	}{
		{"http://localhost:2380", http.StatusForbidden, nil},
	}
	for i, tt := range tests {
		pt := &PeerTransport{peerPipelineRoundTripper: newRespRoundTripper(tt.code, tt.err)}
		picker := newURLPicker(types.MustNewURLs([]string{tt.u}))
		pn := startTestPeerPipeline(pt, picker)

		pn.post([]byte("testdata"))
		pn.stop()

		select {
		case <-pn.errc:
		default:
			t.Fatalf("#%d: cannot receive from errc", i)
		}
	}
}

// (etcd rafthttp.TestStopBlockedPipeline)
func Test_peerPipeline_stop_blocked(t *testing.T) {
	tr := newRoundTripperBlocker()
	pt := &PeerTransport{peerPipelineRoundTripper: tr}

	picker := newURLPicker(types.MustNewURLs([]string{"http://localhost:2380"}))
	pn := startTestPeerPipeline(pt, picker)

	for i := 0; i < connPerPipeline*10; i++ {
		pn.raftMessageChan <- raftpb.Message{}
	}

	donec := make(chan struct{})
	go func() {
		pn.stop()
		donec <- struct{}{}
	}()
	select {
	case <-donec:
	case <-time.After(time.Second):
		t.Fatal("failed to stop pipeline in 1s")
	}
}
