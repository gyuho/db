package rafttest

import (
	"github.com/gyuho/db/raft"
	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.rafttest.node)
type fakeNode struct {
	raftNode raft.Node
	id       uint64

	iface iface

	stopc  chan struct{}
	pausec chan bool

	// stable
	stableStorageInMemory *raft.StorageStableInMemory
	hardState             raftpb.HardState
}
