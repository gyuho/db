package rafthttp

// peer represents remote Raft node. Local Raft node uses peer to send messages to remote peers.
// stream is long-polling connection, always open to transfer messages.
// pipeline is a series of HTTP clients, and sends HTTP requests to remote peers.
// It is only used when the stream has not been established.
//
// (etcd rafthttp.peer)
type peer struct {
}

// peerRemote handles newly joined peers.
//
// (etcd rafthttp.remote)
type peerRemote struct {
}
