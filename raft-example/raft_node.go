package main

import (
	"context"
	"net/http"
	"net/url"

	"github.com/gyuho/db/pkg/fileutil"
	"github.com/gyuho/db/pkg/netutil"
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/rafthttp"
	"github.com/gyuho/db/raftwal"
	"github.com/gyuho/db/raftwal/raftwalpb"
)

type raftNode struct {
	id  uint64
	url url.URL

	peerIDs  []uint64
	peerURLs types.URLs

	walDir  string
	dataDir string

	lastIndex uint64

	storageMemory *raft.StorageStableInMemory
	wal           *raftwal.WAL
	node          raft.Node
	transport     *rafthttp.Transport

	propc   <-chan []byte // propc to receive proposals
	commitc chan<- []byte // commitc to send ready-to-commit entries
	errc    chan<- error

	stopc         chan struct{}
	stopListenerc chan struct{}
	donec         chan struct{}
}

type config struct {
	id  uint64
	url string

	peerIDs  []uint64
	peerURLs []string

	walDir  string
	dataDir string
}

func newRaftNode(cfg config, propc <-chan []byte) *raftNode {
	url := types.MustNewURL(cfg.url)
	purls := types.MustNewURLs(cfg.peerURLs)

	commitc, errc := make(chan []byte), make(chan error)
	rnd := &raftNode{
		id:       cfg.id,
		url:      url,
		peerIDs:  cfg.peerIDs,
		peerURLs: purls,

		walDir:  cfg.walDir,
		dataDir: cfg.dataDir,

		lastIndex: 0,

		storageMemory: raft.NewStorageStableInMemory(),
		wal:           nil,
		node:          nil,
		transport:     nil,

		propc:   propc,
		commitc: commitc,
		errc:    errc,

		stopc:         make(chan struct{}),
		stopListenerc: make(chan struct{}),
		donec:         make(chan struct{}),
	}
	go rnd.start()
	return rnd
}

func (rnd *raftNode) openWAL() *raftwal.WAL {
	if !fileutil.DirHasFiles(rnd.walDir) {
		if err := fileutil.MkdirAll(rnd.walDir); err != nil {
			logger.Panic(err)
		}

		w, err := raftwal.Create(rnd.walDir, nil)
		if err != nil {
			logger.Panic(err)
		}
		w.Close()
	}

	w, err := raftwal.OpenWALWrite(rnd.walDir, raftwalpb.Snapshot{})
	if err != nil {
		logger.Panic(err)
	}
	return w
}

func (rnd *raftNode) replayWAL() *raftwal.WAL {
	w := rnd.openWAL()
	_, hardstate, ents, err := w.ReadAll()
	if err != nil {
		logger.Panic(err)
	}

	rnd.storageMemory.Append(ents...)

	if len(ents) == 0 {
		rnd.commitc <- nil // to inform that commit channel is current
	} else {
		rnd.lastIndex = ents[len(ents)-1].Index
	}

	rnd.storageMemory.SetHardState(hardstate)
	return w
}

func (rnd *raftNode) start() {
	walExist := fileutil.DirHasFiles(rnd.walDir)
	rnd.wal = rnd.replayWAL()

	cfg := &raft.Config{
		ID:                      rnd.id,
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
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
			rnd.transport.AddPeer(types.ID(rnd.peerIDs[i]), rnd.peerURLs.StringSlice())
		}
	}

	go rnd.startRaft()
	go rnd.startServe()
}

func (rnd *raftNode) startRaft() {
}

func (rnd *raftNode) startServe() {
	ln, err := netutil.NewListenerStoppable(rnd.url.Scheme, rnd.url.Host, nil, rnd.stopListenerc)
	if err != nil {
		logger.Panic(err)
	}

	srv := &http.Server{
		Handler: rnd.transport.HTTPHandler(),
	}
	err = srv.Serve(ln)
	select {
	case <-rnd.stopListenerc:
	default:
		logger.Fatalf("failed to serve (%v)", err)
	}
	<-rnd.donec
}

func (rnd *raftNode) stop() {
	rnd.transport.Stop()
	close(rnd.stopc)
	close(rnd.stopListenerc)
	<-rnd.donec
}

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
