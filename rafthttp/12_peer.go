package rafthttp

import (
	"sync"
	"time"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftsnap"
)

// Peer defines peer operations.
//
// (etcd rafthttp.Peer)
type Peer interface {
	sendMessageToPeer(msgs raftpb.Message)
	sendSnapshotToPeer(msgs raftsnap.Message)
	updatePeer(urls types.URLs)

	attachOutgoingConn(conn *outgoingConn)

	activeSince() time.Time

	stop()
}

// peer represents remote Raft node. Local Raft node uses peer to send messages to remote peers.
// stream is long-polling connection, always open to transfer messages.
// pipeline is a series of HTTP clients, and sends HTTP requests to remote peers.
// It is only used when the stream has not been established.
//
// (etcd rafthttp.peer)
type peer struct {
	// id is the id of this remote peer.
	id     types.ID
	status *peerStatus
	r      Raft

	urlPicker    *urlPicker
	streamWriter *streamWriter

	sendc chan raftpb.Message
	recvc chan raftpb.Message
	propc chan raftpb.Message
	stopc chan struct{}

	mu     sync.Mutex
	paused bool
}

func (p *peer) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
}

func (p *peer) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
}
