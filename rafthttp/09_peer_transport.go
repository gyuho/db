package rafthttp

import (
	"net/http"
	"sync"
	"time"

	"github.com/gyuho/db/pkg/probing"
	"github.com/gyuho/db/pkg/tlsutil"
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftsnap"
)

// PeerTransporter defines rafthttp transport layer.
//
// (etcd rafthttp.Transporter)
type PeerTransporter interface {
	// Start starts transporter.
	// Start must be called first.
	//
	// (etcd rafthttp.Transporter.Start)
	Start() error

	// Stop closes all connections and stops the transporter.
	//
	// (etcd rafthttp.Transporter.Stop)
	Stop()

	// HTTPHandler returns http.Handler with '/raft' prefix.
	//
	// (etcd rafthttp.Transporter.Handler)
	HTTPHandler() http.Handler

	// SendMessagesToPeer sends messages to its remote peers.
	//
	// (etcd rafthttp.Transporter.Send)
	SendMessagesToPeer(msgs []raftpb.Message)

	// SendSnapshotToPeer sends snapshot to its remote peers.
	//
	// (etcd rafthttp.Transporter.SendSnapshot)
	SendSnapshotToPeer(msgs raftsnap.Message)

	// AddPeer adds a peer with given peer URLs to the transport.
	//
	// (etcd rafthttp.Transporter.AddPeer)
	AddPeer(id types.ID, urls []string)

	// RemovePeer removes the peer with the given ID.
	//
	// (etcd rafthttp.Transporter.RemovePeer)
	RemovePeer(id types.ID)

	// RemoveAllPeers removes all existing peers in transporter.
	//
	// (etcd rafthttp.Transporter.RemoveAllPeers)
	RemoveAllPeers()

	// UpdatePeer updates the peer with given ID and URLs.
	//
	// (etcd rafthttp.Transporter.UpdatePeer)
	UpdatePeer(id types.ID, urls []string)

	// ActiveSince returns the time that the connection with the peer became active.
	// If the connection is currently inactive, it returns zero time.
	//
	// (etcd rafthttp.Transporter.ActiveSince)
	ActiveSince(id types.ID) time.Time

	// AddPeerRemote adds a remote peer with given URLs.
	//
	// (etcd rafthttp.Transporter.AddRemote)
	AddPeerRemote(id types.ID, urls []string)
}

// PeerTransport implements PeerTransporter. It sends and receives raft messages to/from peers.
//
// (etcd rafthttp.Transport)
type PeerTransport struct {
	TLSInfo     tlsutil.TLSInfo
	DialTimeout time.Duration

	From      types.ID
	ClusterID types.ID
	PeerURLs  types.URLs

	Raft            Raft
	RaftSnapshotter *raftsnap.Snapshotter

	errc chan error

	streamRoundTripper             http.RoundTripper
	peerPipelineRoundTripper       http.RoundTripper
	peerPipelineRoundTripperProber probing.Prober

	mu          sync.RWMutex
	peers       map[types.ID]Peer
	peerRemotes map[types.ID]*peerRemote
}

func (pt *PeerTransport) Start() error {
	var err error

	pt.streamRoundTripper, err = NewStreamRoundTripper(pt.TLSInfo, pt.DialTimeout)
	if err != nil {
		return err
	}

	// allow more idle connections
	pt.peerPipelineRoundTripper, err = NewRoundTripper(pt.TLSInfo, pt.DialTimeout)
	if err != nil {
		return err
	}
	pt.peerPipelineRoundTripperProber = probing.NewProber(pt.peerPipelineRoundTripper)

	pt.peers = make(map[types.ID]Peer)
	pt.peerRemotes = make(map[types.ID]*peerRemote)

	return nil
}

//
//
// ↓ need peerPipeline, streamReader, Peer, peer, peerRemote implementation
//
//
//

// CutPeer drops messages to the specified peer.
func (pt *PeerTransport) CutPeer(id types.ID) {
	pt.mu.RLock()
	p, pok := pt.peers[id]
	r, rok := pt.peerRemotes[id]
	pt.mu.RUnlock()

	if pok {
		p.(Pausable).Pause()
	}
	if rok {
		r.Pause()
	}
}

// MendPeer recovers the dropping message to the given peer.
func (pt *PeerTransport) MendPeer(id types.ID) {
	pt.mu.RLock()
	p, pok := pt.peers[id]
	r, rok := pt.peerRemotes[id]
	pt.mu.RUnlock()

	if pok {
		p.(Pausable).Resume()
	}
	if rok {
		r.Resume()
	}
}

func (pt *PeerTransport) Stop() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	for _, p := range pt.peers {

	}
	for _, r := range pt.peerRemotes {

	}

	if tr, ok := pt.streamRoundTripper.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
	if tr, ok := pt.peerPipelineRoundTripper.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
	pt.peerPipelineRoundTripperProber.RemoveAll()

	pt.peers = nil
	pt.peerRemotes = nil

	return
}

func (pt *PeerTransport) HTTPHandler() http.Handler {

	return nil
}

func (pt *PeerTransport) SendMessagesToPeer(msgs []raftpb.Message) {

	return
}

func (pt *PeerTransport) SendSnapshotToPeer(msgs raftsnap.Message) {

	return
}

func (pt *PeerTransport) AddPeer(id types.ID, urls []string) {

	return
}

func (pt *PeerTransport) RemovePeer(id types.ID) {

	return
}

func (pt *PeerTransport) RemoveAllPeers() {

	return
}

func (pt *PeerTransport) UpdatePeer(id types.ID, urls []string) {

	return
}

func (pt *PeerTransport) ActiveSince(id types.ID) time.Time {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if p, ok := pt.peers[id]; ok {
		return p.activeSince()
	}
	return time.Time{}
}

func (pt *PeerTransport) AddPeerRemote(id types.ID, urls []string) {

	return
}
