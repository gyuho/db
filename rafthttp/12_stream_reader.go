package rafthttp

import (
	"io"
	"sync"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
)

// streamReader reads messages from remote peers.
//
// (etcd rafthttp.streamReader)
type streamReader struct {
	peerID types.ID
	status *peerStatus

	picker        *urlPicker
	peerTransport *PeerTransport

	recvc chan<- raftpb.Message
	propc chan<- raftpb.Message
	stopc chan struct{}
	donec chan struct{}
	errc  chan<- error

	mu     sync.Mutex
	paused bool
	cancel func()
	closer io.Closer
}
