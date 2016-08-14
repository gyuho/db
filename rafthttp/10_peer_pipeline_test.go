package rafthttp

import (
	"testing"

	"github.com/gyuho/db/pkg/scheduleutil"
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
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
	picker := newURLPicker(types.MustNewURLs([]string{"http://localhost:2380"}))
	pt := &PeerTransport{peerPipelineRoundTripper: tr}
	pl := startTestPeerPipeline(pt, picker)

	pl.raftMessageChan <- raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND}

	scheduleutil.WaitSchedule()

	pl.stop()

	if tr.Request() == nil {
		t.Fatal("sender fails to post the data")
	}
}

// (etcd rafthttp.TestPipelineKeepSendingWhenPostError)
func Test_peerPipeline_send_error(t *testing.T) {

}

// (etcd rafthttp.TestPipelineExceedMaximumServing)
func Test_peerPipeline_send_maximum(t *testing.T) {

}

// (etcd rafthttp.TestPipelineSendFailed)
func Test_peerPipeline_send_failed(t *testing.T) {

}

// (etcd rafthttp.TestPipelinePost)
func Test_peerPipeline_send_post(t *testing.T) {

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
