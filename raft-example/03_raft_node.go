package main

import (
	"net/url"
	"path/filepath"

	"github.com/gyuho/db/pkg/fileutil"
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft"
	"github.com/gyuho/db/rafthttp"
	"github.com/gyuho/db/raftsnap"
)

type config struct {
	id               uint64
	clientURL        string
	advertisePeerURL string

	peerIDs  []uint64
	peerURLs []string

	dir string
}

type raftNode struct {
	id               uint64
	clientURL        url.URL
	advertisePeerURL url.URL

	peerIDs  []uint64
	peerURLs types.URLs

	dir     string
	walDir  string
	snapDir string

	snapCount uint64

	electionTickN  int
	heartbeatTickN int

	lastIndex uint64

	storageMemory *raft.StorageStableInMemory
	storage       Storage

	node      raft.Node
	transport *rafthttp.Transport

	// shared channel with dataStore
	propc   chan []byte // propc to receive proposals "FROM"
	commitc chan []byte // commitc to send ready-to-commit entries "TO"
	///////////////////////////////

	errc          chan error
	stopc         chan struct{}
	stopListenerc chan struct{}
	donec         chan struct{}

	ds *dataStore
}

func startRaftNode(cfg config) *raftNode {
	rnd := &raftNode{
		id:               cfg.id,
		clientURL:        types.MustNewURL(cfg.clientURL),
		advertisePeerURL: types.MustNewURL(cfg.advertisePeerURL),

		peerIDs:  cfg.peerIDs,
		peerURLs: types.MustNewURLs(cfg.peerURLs),

		dir:     cfg.dir,
		walDir:  filepath.Join(cfg.dir, "wal"),
		snapDir: filepath.Join(cfg.dir, "snap"),

		snapCount: 10000,

		electionTickN:  10,
		heartbeatTickN: 1,

		lastIndex: 0,

		storageMemory: raft.NewStorageStableInMemory(),
		storage:       nil,

		node:      nil,
		transport: nil,

		// shared channel with dataStore
		propc:   make(chan []byte, 100),
		commitc: make(chan []byte, 100),
		///////////////////////////////

		errc:          make(chan error),
		stopc:         make(chan struct{}),
		stopListenerc: make(chan struct{}),
		donec:         make(chan struct{}),
	}
	rnd.ds = newDataStore(rnd.propc, rnd.commitc)

	go rnd.start()

	return rnd
}

func (rnd *raftNode) start() {
	logger.Printf("raftNode.start %s at %s", types.ID(rnd.id), rnd.dir)
	if !fileutil.ExistFileOrDir(rnd.snapDir) {
		if err := fileutil.MkdirAll(rnd.snapDir); err != nil {
			panic(err)
		}
	}

	walExist := fileutil.DirHasFiles(rnd.walDir) // MUST BE BEFORE replayWAL
	rnd.storage = newStorage(rnd.replayWAL(), raftsnap.New(rnd.snapDir))

	cfg := &raft.Config{
		ID:                      rnd.id,
		ElectionTickNum:         rnd.electionTickN,
		HeartbeatTimeoutTickNum: rnd.heartbeatTickN,
		StorageStable:           rnd.storageMemory,
		MaxEntryNumPerMsg:       1024 * 1024,
		MaxInflightMsgNum:       256,
	}

	if walExist {
		rnd.node = raft.RestartNode(cfg)
	} else {
		raftPeers := make([]raft.Peer, len(rnd.peerIDs))
		for i, id := range rnd.peerIDs {
			raftPeers[i] = raft.Peer{ID: id}
		}
		rnd.node = raft.StartNode(cfg, raftPeers)
	}

	rnd.transport = &rafthttp.Transport{
		Sender:    types.ID(rnd.id),
		ClusterID: 0x1000,
		Raft:      rnd,
		Errc:      make(chan error),
	}
	rnd.transport.Start()

	for i := range rnd.peerIDs {
		if rnd.peerIDs[i] != rnd.id { // do not add self as peer
			rnd.transport.AddPeer(types.ID(rnd.peerIDs[i]), []string{rnd.peerURLs[i].String()})
		}
	}

	go rnd.startRaftHandler()
	go rnd.startPeerHandler()
}

func (rnd *raftNode) stop() {
	logger.Warningln("stopping transport...")
	rnd.transport.Stop()

	logger.Warningln("closing stopc...")
	select {
	case <-rnd.stopc:
	default:
		close(rnd.stopc)
	}

	logger.Warningln("closing stopListenerc...")
	select {
	case <-rnd.stopListenerc:
	default:
		close(rnd.stopListenerc)
	}

	logger.Warningln("closing donec")
	select {
	case <-rnd.donec:
	default:
		close(rnd.donec)
	}
}
