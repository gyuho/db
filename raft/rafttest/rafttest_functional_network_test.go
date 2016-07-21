package rafttest

import (
	"testing"
	"time"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.rafttest.TestNetworkDrop)
func Test_network_drop(t *testing.T) {
	// drop around 10% messages
	sent := 1000
	droprate := 0.1
	fn := newFakeNetwork(1, 2)
	fn.dropConnectionByPercentage(1, 2, droprate)
	for i := 0; i < sent; i++ {
		fn.send(raftpb.Message{From: 1, To: 2})
	}

	c := fn.recvFrom(2)

	received := 0
	done := false
	for !done {
		select {
		case <-c:
			received++
		default:
			done = true
		}
	}

	drop := sent - received
	if drop > int((droprate+0.1)*float64(sent)) || drop < int((droprate-0.1)*float64(sent)) {
		t.Fatalf("expected around %.2f, got %d", droprate*float64(sent), drop)
	}
}

// (etcd raft.rafttest.TestNetworkDelay)
func Test_network_delay(t *testing.T) {
	sent := 1000
	delay := time.Millisecond
	delayrate := 0.1
	fn := newFakeNetwork(1, 2)

	fn.delayConnectionByPercentage(1, 2, delay, delayrate)
	var total time.Duration
	for i := 0; i < sent; i++ {
		s := time.Now()
		fn.send(raftpb.Message{From: 1, To: 2})
		total += time.Since(s)
	}

	w := time.Duration(float64(sent)*delayrate/2) * delay
	if total < w {
		t.Fatalf("expected > %v, total %v", w, total)
	}
}
