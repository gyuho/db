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

// (etcd etcdserver.EtcdServer.applySnapshot)
func (rnd *raftNode) applySnapshot(ap *apply) {
	if raftpb.IsEmptySnapshot(ap.snapshotToSave) {
		return
	}

	// TODO: progress, backend

	// if ap.snapshotToSave.Metadata.Index <=
}

// (etcd etcdserver.EtcdServer.applyEntries,apply)
func (rnd *raftNode) applyEntries(ap *apply) {
	if len(ap.entriesToApply) == 0 {
		return
	}

	for i := range ap.entriesToApply {
		switch ap.entriesToApply[i].Type {
		case raftpb.ENTRY_TYPE_NORMAL:
			if len(ap.entriesToApply[i].Data) == 0 {
				// ignore empty message
				break
			}
			select {
			case rnd.commitc <- ap.entriesToApply[i].Data:
			case <-rnd.stopc:
				return
			}

		case raftpb.ENTRY_TYPE_CONFIG_CHANGE:
			// TODO
		}

		if ap.entriesToApply[i].Index == rnd.lastIndex { // special nil commit to signal that replay has finished
			select {
			case rnd.commitc <- nil:
			case <-rnd.stopc:
				return
			}
		}
	}
}

// (etcd etcdserver.EtcdServer.applyAll)
func (rnd *raftNode) applyAll(ap *apply) {
	rnd.applySnapshot(ap)
	rnd.applyEntries(ap)

	<-ap.readyToSnapshot

	// TODO: trigger snapshot
}

type apply struct {
	entriesToApply  []raftpb.Entry  // (etcd etcdserver.apply.entries)
	snapshotToSave  raftpb.Snapshot // (etcd etcdserver.apply.snapshot)
	readyToSnapshot chan struct{}   // (etcd etcdserver.apply.raftDone)
}

// (etcd etcdserver.raftNode.start, contrib.raftexample.raftNode.serveChannels)
func (rnd *raftNode) startRaftHandler() {
	defer rnd.storage.Close()

	ticker := time.NewTicker(time.Duration(rnd.electionTickN) * time.Millisecond)
	defer ticker.Stop()

	go rnd.handleProposal()

	// handle Ready
	for {
		select {
		case <-ticker.C:
			rnd.node.Tick()

		case rd := <-rnd.node.Ready(): // ready to commit
			isLeader := false
			if rd.SoftState != nil {
				if rd.SoftState.NodeState == raftpb.NODE_STATE_LEADER {
					isLeader = true
				}
			}

			readyToSnapshot := make(chan struct{})
			ap := &apply{
				entriesToApply:  rd.EntriesToApply,
				snapshotToSave:  rd.SnapshotToSave,
				readyToSnapshot: readyToSnapshot,
			}
			go rnd.applyAll(ap)

			// (Raft §10.2.1 Writing to the leader’s disk in parallel, p.141)
			// leader writes the new log entry to disk before replicating the entry
			// to its followers. Then, the followers write the entry to their disks.
			// Fortunately, the leader can write to its disk in parallel with replicating
			// to the followers and them writing to their disks.
			if isLeader {
				rnd.transport.Send(rd.MessagesToSend)
			}

			// etcdserver/raft.go: r.storage.Save(rd.HardState, rd.Entries)
			if err := rnd.storage.Save(rd.HardStateToSave, rd.EntriesToAppend); err != nil {
				panic(err)
			}

			if !raftpb.IsEmptySnapshot(rd.SnapshotToSave) {
				// etcdserver/raft.go: r.storage.SaveSnap(rd.Snapshot)
				if err := rnd.storage.SaveSnap(rd.SnapshotToSave); err != nil {
					panic(err)
				}

				// etcdserver/raft.go: r.raftStorage.ApplySnapshot(rd.Snapshot)
				rnd.storageMemory.ApplySnapshot(rd.SnapshotToSave)
			}

			// etcdserver/raft.go: r.raftStorage.Append(rd.Entries)
			rnd.storageMemory.Append(rd.EntriesToAppend...)

			if !isLeader {
				rnd.transport.Send(rd.MessagesToSend)
			}

			close(readyToSnapshot)

			// after commit, must call Advance
			// etcdserver/raft.go: r.Advance()
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
