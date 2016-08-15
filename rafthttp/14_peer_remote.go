package rafthttp

import "github.com/gyuho/db/pkg/types"

// peerRemote handles newly joined peers.
//
// (etcd rafthttp.remote)
type peerRemote struct {
	peerID types.ID
	status *peerStatus

	pipeline *pipeline
}

func (r *peerRemote) Pause() {
}

func (r *peerRemote) Resume() {
}
