package rafttest

import "github.com/gyuho/db/raft/raftpb"

// (etcd raft.rafttest.iface)
type iface interface {
	send(msg raftpb.Message)
	recv() chan raftpb.Message

	connect()
	disconnect()
}

// (etcd raft.rafttest.nodeNetwork)
type fakeNetworkNode struct {
	id uint64
	*fakeNetwork
}

func (fn *fakeNetwork) fakeNetworkNode(id uint64) iface {
	return &fakeNetworkNode{id: id, fakeNetwork: fn}
}

func (nt *fakeNetworkNode) send(msg raftpb.Message) {
	nt.fakeNetwork.send(msg)
}

func (nt *fakeNetworkNode) recv() chan raftpb.Message {
	return nt.fakeNetwork.recvFrom(nt.id)
}

func (nt *fakeNetworkNode) connect() {
	nt.fakeNetwork.connect(nt.id)
}

func (nt *fakeNetworkNode) disconnect() {
	nt.fakeNetwork.disconnect(nt.id)
}
