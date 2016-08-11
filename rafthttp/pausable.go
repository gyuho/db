package rafthttp

// Pausable defines pausable transport interface.
type Pausable interface {
	Pause()
	Resume()
}
