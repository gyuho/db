package rafthttp

import (
	"net/http"
	"time"

	"github.com/gyuho/db/pkg/probing"
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftsnap"
)

// Get implements peerGetter interface.
func (tr *Transport) Get(id types.ID) Peer {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return tr.peers[id]
}

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
		p.stop()
	}
	for _, r := range tr.peerRemotes {
		r.stop()
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
	mux := http.NewServeMux()
	mux.Handle(PrefixRaftStream+"/", newStreamHandler(tr, tr.Raft, tr, tr.Sender, tr.ClusterID)) // +"/" is necessary
	mux.Handle(PrefixRaftSnapshot, newSnapshotSenderHandler(tr, tr.Raft, tr.RaftSnapshotter, tr.ClusterID))
	mux.Handle(PrefixRaft, newPipelineHandler(tr, tr.Raft, tr.ClusterID))
	mux.Handle(PrefixRaftProbing, probing.NewHTTPHealthHandler())
	return mux
}

func (tr *Transport) Send(msgs []raftpb.Message) {
	for _, msg := range msgs {
		if msg.To == 0 {
			continue // ignore intentionally dropped message
		}
		to := types.ID(msg.To)

		tr.mu.RLock()
		p, pok := tr.peers[to]
		r, rok := tr.peerRemotes[to]
		tr.mu.RUnlock()

		if pok {
			p.sendMessageToPeer(msg)
			continue
		}

		if rok {
			r.send(msg)
			continue
		}

		logger.Infof("ignored message %q to unknown peer %s", msg.Type, to)
	}
}

func (tr *Transport) SendSnapshot(msg raftsnap.Message) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	to := tr.peers[types.ID(msg.To)]
	if to == nil {
		msg.CloseWithError(ErrMemberNotFound)
		return
	}

	to.sendSnapshotToPeer(msg)
}

func (tr *Transport) AddPeer(peerID types.ID, urls []string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.peers == nil {
		panic("transport stopped")
	}

	if _, ok := tr.peers[peerID]; ok {
		return
	}

	us, err := types.NewURLs(urls)
	if err != nil {
		logger.Panicf("types.NewURLs fail %v (%v)", err, urls)
	}

	tr.peers[peerID] = startPeer(tr, peerID, us)

	addPeerToProber(tr.pipelineRoundTripperProber, peerID.String(), urls)

	logger.Infof("added peer %s", peerID)
}

func (tr *Transport) removePeer(peerID types.ID) {
	if peer, ok := tr.peers[peerID]; ok {
		peer.stop()
	} else {
		logger.Panicf("unexpected removal of unknown peer %s", peerID)
	}
	delete(tr.peers, peerID)

	tr.pipelineRoundTripperProber.Remove(peerID.String())

	logger.Infof("removed peer %s", peerID)
}

func (tr *Transport) RemovePeer(peerID types.ID) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.removePeer(peerID)
}

func (tr *Transport) RemoveAllPeers() {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	for id := range tr.peers {
		tr.removePeer(id)
	}
}

func (tr *Transport) UpdatePeer(peerID types.ID, urls []string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if _, ok := tr.peers[peerID]; !ok {
		return
	}

	us, err := types.NewURLs(urls)
	if err != nil {
		logger.Panicf("types.NewURLs fail %v (%v)", err, urls)
	}
	tr.peers[peerID].updatePeer(us)

	tr.pipelineRoundTripperProber.Remove(peerID.String())
	addPeerToProber(tr.pipelineRoundTripperProber, peerID.String(), urls)

	logger.Infof("updated peer %s", peerID)
}

func (tr *Transport) ActiveSince(peerID types.ID) time.Time {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	if p, ok := tr.peers[peerID]; ok {
		return p.activeSince()
	}
	return time.Time{}
}

func (tr *Transport) AddPeerRemote(peerID types.ID, urls []string) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	if tr.peerRemotes == nil {
		// there's no clean way to shutdown the golang http server
		// (see: https://github.com/golang/go/issues/4674) before
		// stopping the transport; ignore any new connections.
		return
	}

	if _, ok := tr.peers[peerID]; ok {
		return
	}
	if _, ok := tr.peerRemotes[peerID]; ok {
		return
	}

	us, err := types.NewURLs(urls)
	if err != nil {
		logger.Panicf("types.NewURLs fail %v (%v)", err, urls)
	}
	tr.peerRemotes[peerID] = startRemote(peerID, us, tr)

	logger.Infof("added peer remote %s", peerID)
}
