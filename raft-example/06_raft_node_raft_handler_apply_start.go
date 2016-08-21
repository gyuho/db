package main

import (
	"time"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
)

// (etcd etcdserver.apply)
type apply struct {
	entriesToApply  []raftpb.Entry  // (etcd etcdserver.apply.entries)
	snapshotToSave  raftpb.Snapshot // (etcd etcdserver.apply.snapshot)
	readyToSnapshot chan struct{}   // (etcd etcdserver.apply.raftDone)
}

// (etcd etcdserver.progress)
type progress struct {
	snapshotIndex uint64
	appliedIndex  uint64
}

// (etcd etcdserver.EtcdServer.applySnapshot)
func (rnd *raftNode) applySnapshot(pr *progress, ap *apply) {
	if raftpb.IsEmptySnapshot(ap.snapshotToSave) {
		return
	}

	// TODO: progress
	// TODO: save to backend
}

// (etcd etcdserver.EtcdServer.applyEntries,apply)
func (rnd *raftNode) applyEntries(pr *progress, ap *apply) {
	if len(ap.entriesToApply) == 0 {
		return
	}

	// TODO: handle progress

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
			// TODO: support config change
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

func (rnd *raftNode) triggerSnapshot(pr *progress) {
	// TODO
}

// (etcd etcdserver.EtcdServer.applyAll)
func (rnd *raftNode) applyAll(pr *progress, ap *apply) {
	rnd.applySnapshot(pr, ap)
	rnd.applyEntries(pr, ap)

	<-ap.readyToSnapshot
	rnd.triggerSnapshot(pr)
}

// (etcd etcdserver.raftNode.start, contrib.raftexample.raftNode.serveChannels)
func (rnd *raftNode) startRaftHandler() {
	snap, err := rnd.storageMemory.Snapshot()
	if err != nil {
		panic(err)
	}
	pr := &progress{
		snapshotIndex: snap.Metadata.Index,
		appliedIndex:  snap.Metadata.Index,
	}

	defer rnd.storage.Close()

	ticker := time.NewTicker(time.Duration(rnd.electionTickN) * time.Millisecond)
	defer ticker.Stop()

	go rnd.handleProposal()

	for {
		select {
		case <-ticker.C:
			rnd.node.Tick()

		case rd := <-rnd.node.Ready():
			isLeader := false
			if rd.SoftState != nil && rd.SoftState.NodeState == raftpb.NODE_STATE_LEADER {
				isLeader = true
			}

			readyToSnapshot := make(chan struct{})
			go rnd.applyAll(pr, &apply{
				entriesToApply:  rd.EntriesToApply,
				snapshotToSave:  rd.SnapshotToSave,
				readyToSnapshot: readyToSnapshot,
			})

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
				if err := rnd.storageMemory.ApplySnapshot(rd.SnapshotToSave); err != nil {
					panic(err)
				}
			}

			// etcdserver/raft.go: r.raftStorage.Append(rd.Entries)
			if err := rnd.storageMemory.Append(rd.EntriesToAppend...); err != nil {
				panic(err)
			}

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