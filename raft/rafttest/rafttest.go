package rafttest

import (
	"math/rand"
	"sync"
	"time"

	"github.com/gyuho/db/raft"
	"github.com/gyuho/db/raft/raftpb"
)

type connection struct {
	from, to uint64
}

type delay struct {
	d    time.Duration
	rate float64
}

// (etcd raft.rafttest.raftNetwork)
type fakeNetwork struct {
	mu                    sync.Mutex
	allDisconnectedIDs    map[uint64]bool
	allDroppedConnections map[connection]float64
	allDelayedConnections map[connection]delay
	recvQueues            map[uint64]chan raftpb.Message
}

// (etcd raft.rafttest.newRaftNetwork)
func newFakeNetwork(ids ...uint64) *fakeNetwork {
	fn := &fakeNetwork{
		allDisconnectedIDs:    make(map[uint64]bool),
		allDroppedConnections: make(map[connection]float64),
		allDelayedConnections: make(map[connection]delay),
		recvQueues:            make(map[uint64]chan raftpb.Message),
	}

	for _, id := range ids {
		fn.recvQueues[id] = make(chan raftpb.Message, 1024)
	}
	return fn
}

// (etcd raft.rafttest.network)
type network interface {
	// 1.0 drops all messages
	dropConnectionByPercentage(from, to uint64, percentage float64)

	// delay message for (0, d] randomly at given rate (1.0 delay all messages)
	delayConnectionByPercentage(from, to uint64, d time.Duration, rate float64)

	recoverAll()

	connect(id uint64)
	disconnect(id uint64)
}

func (fn *fakeNetwork) dropConnectionByPercentage(from, to uint64, percentage float64) {
	fn.mu.Lock()
	fn.allDroppedConnections[connection{from, to}] = percentage
	fn.mu.Unlock()
}

func (fn *fakeNetwork) delayConnectionByPercentage(from, to uint64, d time.Duration, rate float64) {
	fn.mu.Lock()
	fn.allDelayedConnections[connection{from, to}] = delay{d, rate}
	fn.mu.Unlock()
}

func (fn *fakeNetwork) recoverAll() {
	fn.mu.Lock()
	fn.allDroppedConnections = make(map[connection]float64)
	fn.allDelayedConnections = make(map[connection]delay)
	fn.mu.Unlock()
}

func (fn *fakeNetwork) disconnect(id uint64) {
	fn.mu.Lock()
	fn.allDisconnectedIDs[id] = true
	fn.mu.Unlock()
}

func (fn *fakeNetwork) connect(id uint64) {
	fn.mu.Lock()
	fn.allDisconnectedIDs[id] = false
	fn.mu.Unlock()
}

func (fn *fakeNetwork) send(msg raftpb.Message) {
	fn.mu.Lock()
	to := fn.recvQueues[msg.To]
	if fn.allDisconnectedIDs[msg.To] {
		to = nil
	}
	droppedPerc := fn.allDroppedConnections[connection{msg.From, msg.To}]
	delayedConn := fn.allDelayedConnections[connection{msg.From, msg.To}]
	fn.mu.Unlock()

	if to == nil {
		return
	}
	if droppedPerc != 0 && rand.Float64() < droppedPerc {
		return
	}

	if delayedConn.d != 0 && rand.Float64() < delayedConn.rate {
		rd := rand.Int63n(int64(delayedConn.d))
		time.Sleep(time.Duration(rd))
	}

	select {
	case to <- msg:
	default: // drop the message when receiver queue is full
	}
}

func (fn *fakeNetwork) recvFrom(from uint64) chan raftpb.Message {
	fn.mu.Lock()
	fromc := fn.recvQueues[from]
	if fn.allDisconnectedIDs[from] {
		fromc = nil
	}
	fn.mu.Unlock()

	return fromc
}

// (etcd raft.rafttest.iface)
type iface interface {
	send(msg raftpb.Message)
	recv() chan raftpb.Message

	connect()
	disconnect()
}

func (fn *fakeNetwork) nodeFakeNetwork(id uint64) iface {
	return &nodeFakeNetwork{id: id, fakeNetwork: fn}
}

// (etcd raft.rafttest.nodeNetwork)
type nodeFakeNetwork struct {
	id uint64
	*fakeNetwork
}

func (nt *nodeFakeNetwork) send(msg raftpb.Message) {
	nt.fakeNetwork.send(msg)
}

func (nt *nodeFakeNetwork) recv() chan raftpb.Message {
	return nt.fakeNetwork.recvFrom(nt.id)
}

func (nt *nodeFakeNetwork) connect() {
	nt.fakeNetwork.connect(nt.id)
}

func (nt *nodeFakeNetwork) disconnect() {
	nt.fakeNetwork.disconnect(nt.id)
}

// (etcd raft.rafttest.node)
type testNode struct {
	raftNode raft.Node
	id       uint64

	iface iface

	stopc  chan struct{}
	pausec chan bool

	// stable
	stableStorageInMemory *raft.StorageStableInMemory
	hardState             raftpb.HardState
}
