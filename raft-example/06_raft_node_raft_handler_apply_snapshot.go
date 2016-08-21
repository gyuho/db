package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	configState   raftpb.ConfigState
	snapshotIndex uint64
	appliedIndex  uint64
}

// (etcd etcdserver.EtcdServer.applySnapshot)
func (rnd *raftNode) applySnapshot(pr *progress, ap *apply) {
	if raftpb.IsEmptySnapshot(ap.snapshotToSave) {
		return
	}

	logger.Infof("applying snapshot at index %d", pr.snapshotIndex)
	defer logger.Infof("finished applying snapshot at index %d", pr.snapshotIndex)

	if ap.snapshotToSave.Metadata.Index <= pr.appliedIndex {
		logger.Panicf("snapshot index [%d] should > progress.appliedIndex [%d] + 1", ap.snapshotToSave.Metadata.Index, pr.appliedIndex)
	}

	dbFilePath, err := rnd.storage.DBFilePath(ap.snapshotToSave.Metadata.Index)
	if err != nil {
		panic(err)
	}
	fpath := filepath.Join(rnd.snapDir, "db")
	if err := os.Rename(dbFilePath, fpath); err != nil {
		panic(err)
	}
	fmt.Println(fpath)

	// TODO: save to backend

	pr.configState = ap.snapshotToSave.Metadata.ConfigState
	pr.snapshotIndex = ap.snapshotToSave.Metadata.Index
	pr.appliedIndex = ap.snapshotToSave.Metadata.Index
}

// (etcd etcdserver.EtcdServer.applyEntries,apply)
func (rnd *raftNode) applyEntries(pr *progress, ap *apply) {
	if len(ap.entriesToApply) == 0 {
		return
	}

	firstIdx := ap.entriesToApply[0].Index
	if firstIdx > pr.appliedIndex+1 {
		logger.Panicf("first index of committed entry[%d] should <= progress.appliedIndex[%d] + 1", firstIdx, pr.appliedIndex)
	}

	var ents []raftpb.Entry
	if pr.appliedIndex-firstIdx+1 < uint64(len(ap.entriesToApply)) {
		ents = ap.entriesToApply[pr.appliedIndex-firstIdx+1:]
	}
	if len(ents) == 0 {
		return
	}

	for i := range ents {
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
			var cc raftpb.ConfigChange
			cc.Unmarshal(ap.entriesToApply[i].Data)
			rnd.node.ApplyConfigChange(cc)
			switch cc.Type {
			case raftpb.CONFIG_CHANGE_TYPE_ADD_NODE:
				if len(cc.Context) > 0 {
					rnd.transport.AddPeer(types.ID(cc.NodeID), []string{string(cc.Context)})
				}
			case raftpb.CONFIG_CHANGE_TYPE_REMOVE_NODE:
				if cc.NodeID == rnd.id {
					logger.Warningln("%s had already been removed!", types.ID(rnd.id))
					return
				}
				rnd.transport.RemovePeer(types.ID(cc.NodeID))
			}
		}

		// after commit, update appliedIndex
		pr.appliedIndex = ap.entriesToApply[i].Index

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
func (rnd *raftNode) applyAll(pr *progress, ap *apply) {
	rnd.applySnapshot(pr, ap)
	rnd.applyEntries(pr, ap)
	close(ap.readyToSnapshot)
}

func (rnd *raftNode) createSnapshot() {
	// TODO
}

func (rnd *raftNode) triggerSnapshot(pr *progress) {
	if !rnd.periodicSnapshot && pr.appliedIndex-pr.snapshotIndex <= rnd.snapCount {
		return
	}

	logger.Infof("start snapshot [applied index: %d | last snapshot index: %d]", pr.appliedIndex, pr.snapshotIndex)
	rnd.createSnapshot() // TODO
	pr.snapshotIndex = pr.appliedIndex
}

// (etcd etcdserver.raftNode.start, contrib.raftexample.raftNode.serveChannels)
func (rnd *raftNode) startRaftHandler() {
	snap, err := rnd.storageMemory.Snapshot()
	if err != nil {
		panic(err)
	}
	pr := &progress{
		configState:   snap.Metadata.ConfigState,
		snapshotIndex: snap.Metadata.Index,
		appliedIndex:  snap.Metadata.Index,
	}

	defer rnd.storage.Close()

	ticker := time.NewTicker(time.Duration(rnd.electionTickN) * time.Millisecond)
	defer ticker.Stop()

	tickerSnap := time.NewTicker(rnd.snapshotInterval)
	defer tickerSnap.Stop()

	go rnd.handleProposal()

	for {
		select {
		case <-ticker.C:
			rnd.node.Tick()

		case <-tickerSnap.C:
			rnd.triggerSnapshot(pr)

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

			// wait for the raft routine to finish the disk writes before triggering a
			// snapshot. or applied index might be greater than the last index in raft
			// storage, since the raft routine might be slower than apply routine.
			<-readyToSnapshot
			rnd.triggerSnapshot(pr)

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
