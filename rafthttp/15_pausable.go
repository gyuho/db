package rafthttp

// Pausable defines pausable transport interface.
//
// (etcd rafthttp.Pausable)
type Pausable interface {
	Pause()
	Resume()
}
