package rafthttp

import (
	"sync"
	"time"
)

// Peer defines peer operations.
//
// (etcd rafthttp.Peer)
type Peer interface {
	activeSince() time.Time
}

// peer represents remote Raft node. Local Raft node uses peer to send messages to remote peers.
// stream is long-polling connection, always open to transfer messages.
// pipeline is a series of HTTP clients, and sends HTTP requests to remote peers.
// It is only used when the stream has not been established.
//
// (etcd rafthttp.peer)
type peer struct {
	mu sync.Mutex
}

func (p *peer) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
}

func (p *peer) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
}
