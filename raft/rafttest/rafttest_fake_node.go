package rafttest

import (
	"context"
	"log"
	"time"

	"github.com/gyuho/db/raft"
	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.rafttest.node)
type fakeNode struct {
	raft.Node
	id uint64

	// stable
	stableStorageInMemory *raft.StorageStableInMemory
	hardState             raftpb.HardState

	iface iface

	stopc  chan struct{}
	pausec chan bool
}

// (etcd raft.rafttest.startNode)
func startFakeNode(id uint64, peers []raft.Peer, iface iface) *fakeNode {
	st := raft.NewStorageStableInMemory()

	nd := raft.StartNode(&raft.Config{
		ID:                      id,
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		StorageStable:           st,
		MaxEntryNumPerMsg:       1024 * 1024,
		MaxInflightMsgNum:       256,
	}, peers)

	fnd := &fakeNode{
		Node: nd,
		id:   id,

		stableStorageInMemory: st,

		iface: iface,

		pausec: make(chan bool),
	}

	fnd.start()

	return fnd
}

func (fnd *fakeNode) start() {
	fnd.stopc = make(chan struct{})
	ticker := time.Tick(5 * time.Millisecond)

	go func() {
		for {
			select {
			case <-ticker:
				fnd.Tick()

			case rd := <-fnd.Ready():
				if !raftpb.IsEmptyHardState(rd.HardStateToSave) {
					fnd.hardState = rd.HardStateToSave
					fnd.stableStorageInMemory.SetHardState(fnd.hardState)
				}
				fnd.stableStorageInMemory.Append(rd.EntriesToSave...)

				time.Sleep(time.Millisecond)

				for _, msg := range rd.MessagesToSend {
					fnd.iface.send(msg)
				}

				fnd.Advance()

			case msg := <-fnd.iface.recv():
				fnd.Step(context.TODO(), msg)

			case <-fnd.stopc:
				fnd.Stop()
				log.Printf("raft.%d: stopped", fnd.id)
				fnd.Node = nil
				close(fnd.stopc)
				return

			case p := <-fnd.pausec:
				var receivedMsgs []raftpb.Message
				for p {
					select {
					case msg := <-fnd.iface.recv():
						receivedMsgs = append(receivedMsgs, msg)
					case p = <-fnd.pausec:
					}
				}

				// step all pending messages
				for _, m := range receivedMsgs {
					fnd.Step(context.TODO(), m)
				}
			}
		}
	}()
}

func (fnd *fakeNode) pause() {
	fnd.pausec <- true
}

func (fnd *fakeNode) resume() {
	fnd.pausec <- false
}

func (fnd *fakeNode) stop() {
	fnd.iface.disconnect()
	fnd.stopc <- struct{}{}

	// wait for the shutdown
	<-fnd.stopc
}

func (fnd *fakeNode) restart() {
	// wait for the shutdown
	<-fnd.stopc

	fnd.Node = raft.RestartNode(&raft.Config{
		ID:                      fnd.id,
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		StorageStable:           fnd.stableStorageInMemory,
		MaxEntryNumPerMsg:       1024 * 1024,
		MaxInflightMsgNum:       256,
	})

	fnd.start()

	fnd.iface.connect()
}
