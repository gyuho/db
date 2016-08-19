package rafthttp

import (
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
)

// peerRemote handles newly joined peers.
// It is the leader from the newly joined peer node's viewpoint.
//
// (etcd rafthttp.remote)
type peerRemote struct {
	peerID types.ID
	status *peerStatus

	pipeline *pipeline
}

func (r *peerRemote) stop() {
	r.pipeline.stop()
}

func (r *peerRemote) Pause() {
	r.pipeline.stop()
}

func (r *peerRemote) Resume() {
	r.pipeline.start()
}

// (etcd rafthttp.startRemote)
func startRemote(peerID types.ID, peerURLs types.URLs, tr *Transport) *peerRemote {
	pipeline := &pipeline{
		peerID:    peerID,
		status:    newPeerStatus(peerID),
		r:         tr.Raft,
		picker:    newURLPicker(peerURLs),
		transport: tr,
		errc:      tr.errc,
	}

	pipeline.start()

	return &peerRemote{
		peerID:   peerID,
		status:   pipeline.status,
		pipeline: pipeline,
	}
}

func (r *peerRemote) send(msg raftpb.Message) {
	select {
	case r.pipeline.msgc <- msg:
	default:
		logger.Warningf("dropped %q from %s since sending buffer is full", msg.Type, types.ID(msg.From))
		if r.status.isActive() {
			logger.Warningf("%s network is bad/overloaded", r.peerID)
		}
	}
}
