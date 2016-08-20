package main

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/gyuho/db/pkg/fileutil"
	"github.com/gyuho/db/pkg/netutil"
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/rafthttp"
	"github.com/gyuho/db/raftwal"
	"github.com/gyuho/db/raftwal/raftwalpb"
)

type config struct {
	id  uint64
	url string

	peerIDs  []uint64
	peerURLs []string

	dir string
}

type raftNode struct {
	id  uint64
	url url.URL

	peerIDs  []uint64
	peerURLs types.URLs

	walDir  string
	snapDir string

	electionTickN  int
	heartbeatTickN int

	lastIndex uint64

	storageMemory *raft.StorageStableInMemory
	wal           *raftwal.WAL
	node          raft.Node
	transport     *rafthttp.Transport

	// shared channel with dataStore
	propc   chan []byte // propc to receive proposals "FROM"
	commitc chan []byte // commitc to send ready-to-commit entries "TO"
	errc    chan error
	///////////////////////////////

	stopc         chan struct{}
	stopListenerc chan struct{}
	donec         chan struct{}
}

func newRaftNode(cfg config, propc, commitc chan []byte, errc chan error) *raftNode {
	rnd := &raftNode{
		id:       cfg.id,
		url:      types.MustNewURL(cfg.url),
		peerIDs:  cfg.peerIDs,
		peerURLs: types.MustNewURLs(cfg.peerURLs),

		walDir:  filepath.Join(cfg.dir, "wal"),
		snapDir: filepath.Join(cfg.dir, "snap"),

		electionTickN:  10,
		heartbeatTickN: 1,

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
			rnd.transport.AddPeer(types.ID(rnd.peerIDs[i]), rnd.peerURLs.StringSlice())
		}
	}

	go rnd.startRaft()
	go rnd.startServe()
}

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
		case raftpb.ENTRY_TYPE_CONFIG_CHANGE: // TODO

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

func (rnd *raftNode) startRaft() {
	defer rnd.wal.Close()

	ticker := time.NewTicker(time.Duration(rnd.electionTickN) * time.Millisecond)
	defer ticker.Stop()

	go rnd.handleProposal()

	// handle Ready
	for {
		select {
		case <-ticker.C:
			rnd.node.Tick()

		case rd := <-rnd.node.Ready():
			rnd.wal.Save(rd.HardStateToSave, rd.EntriesToAppend)
			rnd.storageMemory.Append(rd.EntriesToAppend...)
			rnd.transport.Send(rd.MessagesToSend)

			// handle already-committed entries
			if ok := rnd.handleEntriesToCommit(rd.EntriesToCommit); !ok {
				logger.Warningln("stopping...")
				rnd.stop()
				return
			}

			rnd.node.Advance()

		case err := <-rnd.transport.Errc:
			rnd.errc <- err
			logger.Warningln("stopping;", err)
			rnd.stop()
			return

		case <-rnd.stopc:
			return
		}
	}
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
