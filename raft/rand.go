package raft

import (
	"math/rand"
	"sync"
	"time"
)

/*
https://github.com/coreos/etcd/commit/4a33aa3917eca1a2431ab2e6bd72047f8a659ba7

rand.NewSource creates a 4872 byte object. With a small number of raft
groups in a process this isn't a problem. With 10k raft groups we'd use
46MB for these random sources. The only usage is in
raft.resetRandomizedElectionTimeout which isn't performance critical.
*/

// lockedRand is a small wrapper around rand.Rand to provide
// synchronization. Only the methods needed by the code are exposed
// (e.g. Intn).
type lockedRand struct {
	mu   sync.Mutex
	rand *rand.Rand
}

func (r *lockedRand) Intn(n int) int {
	r.mu.Lock()
	v := r.rand.Intn(n)
	r.mu.Unlock()
	return v
}

var globalRand = &lockedRand{
	rand: rand.New(rand.NewSource(time.Now().UnixNano())),
}
