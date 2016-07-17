package raft

import (
	"sort"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.raft.nodes)
func (rnd *raftNode) allNodeIDs() []uint64 {
	allNodeIDs := make([]uint64, 0, len(rnd.allProgresses))
	for id := range rnd.allProgresses {
		allNodeIDs = append(allNodeIDs, id)
	}
	sort.Sort(uint64Slice(allNodeIDs))
	return allNodeIDs
}

// (etcd raft.raft.addNode)
func (rnd *raftNode) addNode(id uint64) {
	if _, ok := rnd.allProgresses[id]; ok {
		// ???
		raftLogger.Infof("%s ignores redundant 'addNode' call to %x (can happen when initial boostrapping entries are applied twice)", rnd.describe(), id)
		return
	}

	rnd.updateProgress(id, 0, rnd.storageRaftLog.lastIndex()+1)
	rnd.pendingConfigExist = false // ???
}

// (etcd raft.raft.removeNode)
func (rnd *raftNode) deleteNode(id uint64) {
	rnd.deleteProgress(id)
	rnd.pendingConfigExist = false // ???

	if len(rnd.allProgresses) == 0 {
		// ???
		raftLogger.Infof("%s has no progresses when raftNode.deleteNode(%x)... returning...", rnd.describe(), id)
		return
	}

	if rnd.state == raftpb.NODE_STATE_LEADER && rnd.leaderTransfereeID == id {
		rnd.stopLeaderTransfer()
	}
}
