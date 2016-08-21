package main

import (
	"context"
	"time"

	"github.com/gyuho/db/pkg/types"
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

func (rnd *raftNode) handleEntriesToCommit(ents []raftpb.Entry) bool {
	for i := range ents {
		switch ents[i].Type {
		case raftpb.ENTRY_TYPE_NORMAL:
			if len(ents[i].Data) == 0 {
				// ignore empty message
				break
			}
			select {
			case rnd.commitc <- ents[i].Data:
			case <-rnd.stopc:
				return false
			}

		case raftpb.ENTRY_TYPE_CONFIG_CHANGE:
			// TODO
		}

		if ents[i].Index == rnd.lastIndex { // special nil commit to signal that replay has finished
			select {
			case rnd.commitc <- nil:
			case <-rnd.stopc:
				return false
			}
		}
	}

	return true
}

func (rnd *raftNode) startRaftHandler() {
	defer rnd.wal.Close()

	ticker := time.NewTicker(time.Duration(rnd.electionTickN) * time.Millisecond)
	defer ticker.Stop()

	go rnd.handleProposal()

	// handle Ready
	for {
		select {
		case <-ticker.C:
			rnd.node.Tick()

		case rd := <-rnd.node.Ready(): // ready to commit
			rnd.wal.Save(rd.HardStateToSave, rd.EntriesToAppend)
			rnd.storageMemory.Append(rd.EntriesToAppend...)
			rnd.transport.Send(rd.MessagesToSend)

			// handle already-committed entries
			if ok := rnd.handleEntriesToCommit(rd.EntriesToCommit); !ok {
				logger.Warningf("stopping %s", types.ID(rnd.id))
				select {
				case <-rnd.stopc:
				default:
					rnd.stop()
				}
				return
			}

			// after commit, must call Advance
			rnd.node.Advance()

		case err := <-rnd.transport.Errc:
			rnd.errc <- err
			logger.Warningln("stopping %s;", types.ID(rnd.id), err)
			select {
			case <-rnd.stopc:
			default:
				rnd.stop()
			}
			return

		case <-rnd.stopc:
			return
		}
	}
}
