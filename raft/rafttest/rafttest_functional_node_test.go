package rafttest

import (
	"context"
	"testing"
	"time"

	"github.com/gyuho/db/raft"
)

// (etcd raft.rafttest.TestBasicProgress)
func Test_basic_progress(t *testing.T) {
	peers := []raft.Peer{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}}
	fn := newFakeNetwork(1, 2, 3, 4, 5)

	var fakeNodes []*fakeNode
	for i := 1; i <= 5; i++ {
		nd := startFakeNode(uint64(i), peers, fn.fakeNetworkNode(uint64(i)))
		fakeNodes = append(fakeNodes, nd)
	}

	time.Sleep(10 * time.Millisecond)

	for i := 0; i < 10000; i++ {
		fakeNodes[0].Propose(context.TODO(), []byte("somedata"))
	}

	time.Sleep(500 * time.Millisecond)

	for _, nd := range fakeNodes {
		nd.stop()

		if nd.hardState.CommittedIndex != 10006 {
			t.Fatalf("CommittedIndex expected 10006, got %d", nd.hardState.CommittedIndex)
		}
	}
}

// (etcd raft.rafttest.TestRestart)
func Test_Restart(t *testing.T) {

}

// (etcd raft.rafttest.TestPause)
func Test_Pause(t *testing.T) {

}
