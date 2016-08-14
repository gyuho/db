package rafthttp

import (
	"sync"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
)

// peerPipeline contains PeerTransport.
// peerPipeline handles a series of HTTP clients, and sends thoses to remote peers.
// It is only used when the stream has not been established.
//
// (etcd rafthttp.pipeline)
type peerPipeline struct {
	peerID types.ID
	status *peerStatus

	r Raft

	picker        *urlPicker
	peerTransport *PeerTransport

	raftMessageChan chan raftpb.Message
	stopc           chan struct{}
	errc            chan struct{}

	wg sync.WaitGroup
}
