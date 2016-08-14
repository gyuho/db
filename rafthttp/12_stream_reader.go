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

	picker *urlPicker
	pt     *PeerTransport

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

func (r *streamReader) pause() {
	r.mu.Lock()
	r.paused = true
	r.mu.Unlock()
}

func (r *streamReader) resume() {
	r.mu.Lock()
	r.paused = false
	r.mu.Unlock()
}

func (r *streamReader) close() {
	if r.closer != nil {
		r.closer.Close()
	}
	r.closer = nil
}

func (r *streamReader) stop() {
	close(r.stopc)

	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
	}
	r.close()
	r.mu.Unlock()

	<-r.donec
}

func (r *streamReader) start() {
	r.stopc = make(chan struct{})
	r.donec = make(chan struct{})
	if r.errc == nil {
		r.errc = r.pt.errc
	}

	go r.run()
}

func (r *streamReader) dial() {

}

func (r *streamReader) decodeLoop(rc io.ReadCloser) {

}

func (r *streamReader) run() {

}
