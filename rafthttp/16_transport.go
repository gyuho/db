package rafthttp

import (
	"net/http"
	"time"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftsnap"
)

//
//
// ↓ need peerPipeline, streamReader, Peer, peer, peerRemote implementation
//
//
//

func (tr *Transport) Stop() {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	for _, p := range tr.peers {
		_ = p
	}
	for _, r := range tr.peerRemotes {
		_ = r
	}

	if tr, ok := tr.streamRoundTripper.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
	if tr, ok := tr.pipelineRoundTripper.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
	tr.pipelineRoundTripperProber.RemoveAll()

	tr.peers = nil
	tr.peerRemotes = nil

	return
}

func (tr *Transport) HTTPHandler() http.Handler {

	return nil
}

func (tr *Transport) SendMessagesToPeer(msgs []raftpb.Message) {

	return
}

func (tr *Transport) SendSnapshotToPeer(msgs raftsnap.Message) {

	return
}

func (tr *Transport) AddPeer(id types.ID, urls []string) {

	return
}

func (tr *Transport) RemovePeer(id types.ID) {

	return
}

func (tr *Transport) RemoveAllPeers() {

	return
}

func (tr *Transport) UpdatePeer(id types.ID, urls []string) {

	return
}

func (tr *Transport) ActiveSince(id types.ID) time.Time {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	if p, ok := tr.peers[id]; ok {
		return p.activeSince()
	}
	return time.Time{}
}

func (tr *Transport) AddPeerRemote(id types.ID, urls []string) {

	return
}
