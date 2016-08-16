package rafthttp

import (
	"context"

	"github.com/gyuho/db/raft/raftpb"
)

//////////////////////////////////////////////////////////////

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

//////////////////////////////////////////////////////////////
