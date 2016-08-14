package rafthttp

import (
	"net/http"
	"sync"
	"time"

	"github.com/gyuho/db/pkg/probing"
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
	From      types.ID
	ClusterID types.ID
	PeerURLs  types.URLs

	Raft            Raft
	RaftSnapshotter *raftsnap.Snapshotter

	errc chan error

	prober                   probing.Prober
	streamRoundTripper       http.RoundTripper
	peerPipelineRoundTripper http.RoundTripper

	mu          sync.RWMutex
	peers       map[types.ID]*peer
	peerRemotes map[types.ID]*peerRemote
}
