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
	for i := 1; i < 6; i++ {
		nd := startFakeNode(uint64(i), peers, fn.fakeNetworkNode(uint64(i)))
		fakeNodes = append(fakeNodes, nd)
	}

	time.Sleep(500 * time.Millisecond)

	propN := 10

	for i := 0; i < propN; i++ {
		fakeNodes[0].Propose(context.TODO(), []byte("testdata"))
	}

	time.Sleep(500 * time.Millisecond)

	for _, nd := range fakeNodes {
		nd.stop()

		if nd.hardState.CommittedIndex != uint64(propN+6) {
			t.Fatalf("CommittedIndex expected %d, got %d", propN+6, nd.hardState.CommittedIndex)
		}
	}
}

// (etcd raft.rafttest.TestRestart)
func Test_Restart(t *testing.T) {
	// TODO

}

// (etcd raft.rafttest.TestPause)
func Test_Pause(t *testing.T) {
	// TODO

}
