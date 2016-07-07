package raft

// Peer contains peer ID and context data.
//
// (etcd raft.Peer)
type Peer struct {
	ID   uint64
	Data []byte
}
