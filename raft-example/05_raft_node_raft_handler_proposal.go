package main

import (
	"context"

	"github.com/gyuho/db/raft/raftpb"
)

// type rafthttp.Raft interface {
// 	Process(ctx context.Context, msg raftpb.Message) error
// 	IsIDRemoved(id uint64) bool
// 	ReportUnreachable(id uint64)
// 	ReportSnapshot(id uint64, status raftpb.SNAPSHOT_STATUS)
// }

func (rnd *raftNode) Process(ctx context.Context, msg raftpb.Message) error {
	return rnd.node.Step(ctx, msg)
}
func (rnd *raftNode) IsIDRemoved(id uint64) bool                              { return false }
func (rnd *raftNode) ReportUnreachable(id uint64)                             {}
func (rnd *raftNode) ReportSnapshot(id uint64, status raftpb.SNAPSHOT_STATUS) {}

func (rnd *raftNode) handleProposal() {
	for rnd.propc != nil {
		select {
		case prop := <-rnd.propc:
			rnd.node.Propose(context.TODO(), prop)

		case <-rnd.stopc:
			rnd.propc = nil
			return
		}
	}
}
