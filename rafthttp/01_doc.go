// Package rafthttp implements HTTP transportation layer for Raft.
package rafthttp

/*
urlPicker picks URL with pinned index.


peerStatus is the status of remote peer in local member's viewpoint.


streamWriter writes messages to the attached outgoingConn.


PeerTransporter defines rafthttp transport layer.
PeerTransport implements PeerTransporter. It sends and receives raft messages to/from peers.
PeerTransport starts with default http.RoundTrippers: one for streams, the other for peerPipeline.
PeerTransport contains remote peers, which contain peerPipeline, which again contains PeerTransport.


peerPipeline contains PeerTransport.
peerPipeline handles a series of HTTP clients, and sends thoses to remote peers.
It is only used when the stream has not been established.


streamReader reads messages from remote peers.
streamReader contains PeerTransport to dial to remote endpoints.


peer represents remote Raft node. Local Raft node uses peer to send messages to remote peers.
stream is long-polling connection, always open to transfer messages.
pipeline is a series of HTTP clients, and sends HTTP requests to remote peers.
It is only used when the stream has not been established.

peerRemote handles newly joined peers.


peerSnapshotSender contains PeerTransport to dial to remote endpoints.
It sends snapshot to peers.
*/
