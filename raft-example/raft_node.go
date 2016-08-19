package main

import (
	"github.com/coreos/etcd/wal"
	"github.com/gyuho/db/raft"
	"github.com/gyuho/db/rafthttp"
)

type raftNode struct {
	id       uint64
	peerIDs  []uint64
	peerURLs []string

	walDir  string
	dataDir string

	lastLogIndex uint64

	storageMemory *raft.StorageStableInMemory
	wal           *wal.WAL
	node          raft.Node
	transport     *rafthttp.Transport

	propc   <-chan []byte // propc to receive proposals
	commitc chan<- []byte // commitc to send ready-to-commit entries
	errc    chan<- error

	stopc chan struct{}
	donec chan struct{}
}

type config struct {
	id       uint64
	peerIDs  []uint64
	peerURLs []string

	walDir  string
	dataDir string
}

func newRaftNode(cfg config, propc <-chan []byte) *raftNode {
	commitc, errc := make(chan []byte), make(chan error)
	rnd := &raftNode{
		id:       cfg.id,
		peerIDs:  cfg.peerIDs,
		peerURLs: cfg.peerURLs,

		walDir:  cfg.walDir,
		dataDir: cfg.dataDir,

		lastLogIndex: 0,

		storageMemory: raft.NewStorageStableInMemory(),
		wal:           nil,
		node:          nil,
		transport:     nil,

		propc:   propc,
		commitc: commitc,
		errc:    errc,

		stopc: make(chan struct{}),
		donec: make(chan struct{}),
	}

	go rnd.start()
	return rnd
}

func (rnd *raftNode) stop() {
	close(rnd.stopc)
	<-rnd.donec
}

func (rnd *raftNode) start() {

}

func (rnd *raftNode) startServe() {

}

func (rnd *raftNode) startRaft() {

}
