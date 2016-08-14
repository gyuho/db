package rafthttp

import (
	"context"

	"github.com/gyuho/db/raft/raftpb"
)

// Raft defines raft interface needed for transportation layer.
//
// (etcd rafthttp.Raft)
type Raft interface {
	Process(ctx context.Context, msg raftpb.Message) error
	IsIDRemoved(id uint64) bool
	ReportUnreachable(id uint64)
	ReportSnapshot(id uint64, status raftpb.SNAPSHOT_STATUS)
}
