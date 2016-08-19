package rafthttp

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/gyuho/db/pkg/probing"
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
)

// (etcd rafthttp.TestTransportSend)
func Test_Transport_send(t *testing.T) {
	peer1 := newFakePeer()
	peer2 := newFakePeer()
	tr := &Transport{
		peers: map[types.ID]Peer{types.ID(1): peer1, types.ID(2): peer2},
	}
	wmsgsIgnored := []raftpb.Message{
		// bad local message
		{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT},
		// bad remote message
		{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, To: 3},
	}
	wmsgsTo1 := []raftpb.Message{
		// good message
		{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, To: 1},
		{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, To: 1},
	}
	wmsgsTo2 := []raftpb.Message{
		// good message
		{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, To: 2},
		{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, To: 2},
	}
	tr.Send(wmsgsIgnored)
	tr.Send(wmsgsTo1)
	tr.Send(wmsgsTo2)

	if !reflect.DeepEqual(peer1.msgs, wmsgsTo1) {
		t.Fatalf("msgs to peer 1 = %+v, want %+v", peer1.msgs, wmsgsTo1)
	}
	if !reflect.DeepEqual(peer2.msgs, wmsgsTo2) {
		t.Fatalf("msgs to peer 2 = %+v, want %+v", peer2.msgs, wmsgsTo2)
	}
}

// (etcd rafthttp.TestTransportCutMend)
func Test_Transport_Cut_Mend(t *testing.T) {
	peer1 := newFakePeer()
	peer2 := newFakePeer()
	tr := &Transport{
		peers: map[types.ID]Peer{types.ID(1): peer1, types.ID(2): peer2},
	}

	tr.CutPeer(types.ID(1))

	wmsgsTo := []raftpb.Message{
		// good message
		{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, To: 1},
		{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, To: 1},
	}

	tr.Send(wmsgsTo)
	if len(peer1.msgs) > 0 {
		t.Fatalf("msgs expected to be ignored, got %+v", peer1.msgs)
	}

	tr.MendPeer(types.ID(1))

	tr.Send(wmsgsTo)
	if !reflect.DeepEqual(peer1.msgs, wmsgsTo) {
		t.Fatalf("msgs to peer 1 = %+v, want %+v", peer1.msgs, wmsgsTo)
	}
}

// (etcd rafthttp.TestTransportAdd)
func Test_Transport_Add(t *testing.T) {
	tr := &Transport{
		streamRoundTripper:         &roundTripperRecorder{},
		pipelineRoundTripperProber: probing.NewProber(nil),
		peers: make(map[types.ID]Peer),
	}
	tr.AddPeer(1, []string{"http://localhost:2380"})

	s, ok := tr.peers[types.ID(1)]
	if !ok {
		tr.Stop()
		t.Fatal("senders[1] is nil, want exists")
	}

	// duplicate AddPeer is ignored
	tr.AddPeer(1, []string{"http://localhost:2380"})
	ns := tr.peers[types.ID(1)]
	if s != ns {
		t.Fatalf("sender = %v, want %v", ns, s)
	}

	tr.Stop()
}

// (etcd rafthttp.TestTransportRemove)
func Test_Transport_Remove(t *testing.T) {
	tr := &Transport{
		streamRoundTripper:         &roundTripperRecorder{},
		pipelineRoundTripperProber: probing.NewProber(nil),
		peers: make(map[types.ID]Peer),
	}
	tr.AddPeer(1, []string{"http://localhost:2380"})
	tr.RemovePeer(types.ID(1))
	defer tr.Stop()

	if _, ok := tr.peers[types.ID(1)]; ok {
		t.Fatalf("senders[1] exists, want removed")
	}
}

// (etcd rafthttp.TestTransportUpdate)
func Test_Transport_Update(t *testing.T) {
	peer := newFakePeer()
	tr := &Transport{
		pipelineRoundTripperProber: probing.NewProber(nil),
		peers: map[types.ID]Peer{types.ID(1): peer},
	}
	u := "http://localhost:2380"
	tr.UpdatePeer(types.ID(1), []string{u})
	wurls := types.MustNewURLs([]string{"http://localhost:2380"})
	if !reflect.DeepEqual(peer.peerURLs, wurls) {
		t.Fatalf("urls = %+v, want %+v", peer.peerURLs, wurls)
	}
}

// (etcd rafthttp.TestTransporterrc)
func Test_Transport_err(t *testing.T) {
	errc := make(chan error, 1)
	tr := &Transport{
		Raft:                       &fakeRaft{},
		streamRoundTripper:         newRespRoundTripper(http.StatusForbidden, nil),
		pipelineRoundTripper:       newRespRoundTripper(http.StatusForbidden, nil),
		pipelineRoundTripperProber: probing.NewProber(nil),
		peers: make(map[types.ID]Peer),
		errc:  errc,
	}
	tr.AddPeer(1, []string{"http://localhost:2380"})
	defer tr.Stop()

	select {
	case <-errc:
		t.Fatal("received unexpected from errc")
	case <-time.After(10 * time.Millisecond):
	}
	tr.peers[1].sendMessageToPeer(raftpb.Message{})

	select {
	case <-errc:
	case <-time.After(1 * time.Second):
		t.Fatal("cannot receive error from errc")
	}
}
