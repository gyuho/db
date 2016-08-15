package rafthttp

import (
	"net/url"
	"time"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftsnap"
)

type fakePeerGetter struct {
	peers map[types.ID]Peer
}

func (pg *fakePeerGetter) Get(id types.ID) Peer { return pg.peers[id] }

type fakePeer struct {
	msgs     []raftpb.Message
	snapMsgs []raftsnap.Message
	peerURLs types.URLs
	connc    chan *outgoingConn
	paused   bool
}

func newFakePeer() *fakePeer {
	fakeURL, _ := url.Parse("http://localhost")
	return &fakePeer{
		connc:    make(chan *outgoingConn, 1),
		peerURLs: types.URLs{*fakeURL},
	}
}

func (pr *fakePeer) sendMessageToPeer(m raftpb.Message) {
	if pr.paused {
		return
	}
	pr.msgs = append(pr.msgs, m)
}

func (pr *fakePeer) sendSnapshotToPeer(msg raftsnap.Message) {
	if pr.paused {
		return
	}
	pr.snapMsgs = append(pr.snapMsgs, msg)
}

func (pr *fakePeer) updatePeer(urls types.URLs)            { pr.peerURLs = urls }
func (pr *fakePeer) attachOutgoingConn(conn *outgoingConn) { pr.connc <- conn }
func (pr *fakePeer) activeSince() time.Time                { return time.Time{} }
func (pr *fakePeer) stop()                                 {}
func (pr *fakePeer) Pause()                                { pr.paused = true }
func (pr *fakePeer) Resume()                               { pr.paused = false }
