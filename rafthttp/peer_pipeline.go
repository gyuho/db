package rafthttp

// peerPipeline contains PeerTransport.
// peerPipeline handles a series of HTTP clients, and sends thoses to remote peers.
// It is only used when the stream has not been established.
//
// (etcd rafthttp.pipeline)
type peerPipeline struct {
}
