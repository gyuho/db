package rafthttp

import (
	"context"
	"sync"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd rafthttp.fakeRaft)
type fakeRaft struct {
	recvc     chan<- raftpb.Message
	removedID uint64
	err       error
}

func (p *fakeRaft) Process(ctx context.Context, m raftpb.Message) error {
	select {
	case p.recvc <- m:
	default:
	}
	return p.err
}

func (p *fakeRaft) IsIDRemoved(id uint64) bool { return id == p.removedID }

func (p *fakeRaft) ReportUnreachable(id uint64) {}

func (p *fakeRaft) ReportSnapshot(id uint64, status raftpb.SNAPSHOT_STATUS) {}

// (etcd rafthttp.fakeWriteFlushCloser)
type fakeWriterFlusherCloser struct {
	mu      sync.Mutex
	written int
	closed  bool
	err     error
}

func (wfc *fakeWriterFlusherCloser) Write(p []byte) (n int, err error) {
	wfc.mu.Lock()
	defer wfc.mu.Unlock()

	wfc.written += len(p)
	return len(p), wfc.err
}

func (wfc *fakeWriterFlusherCloser) Flush() {}

func (wfc *fakeWriterFlusherCloser) Close() error {
	wfc.mu.Lock()
	defer wfc.mu.Unlock()

	wfc.closed = true
	return wfc.err
}

func (wfc *fakeWriterFlusherCloser) getWritten() int {
	wfc.mu.Lock()
	defer wfc.mu.Unlock()

	return wfc.written
}

func (wfc *fakeWriterFlusherCloser) getClosed() bool {
	wfc.mu.Lock()
	defer wfc.mu.Unlock()

	return wfc.closed
}
