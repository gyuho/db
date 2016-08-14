package rafthttp

import (
	"fmt"
	"io/ioutil"
	"testing"

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

}

// (etcd rafthttp.TestPipelinePostErrorc)
func Test_peerPipeline_send_post_error(t *testing.T) {

}

// (etcd rafthttp.TestStopBlockedPipeline)
func Test_peerPipeline_stop_blocked(t *testing.T) {

}
