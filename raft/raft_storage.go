package raft

type raftStorage struct {
	logger Logger

	// committedIndex is the highest log index that is
	// known to be stored in stable storage on majority
	// of cluster (quorum of cluster).
	committedIndex uint64

	// appliedIndex is the highest log index that
	// the application has been instructed to apply
	// to its state machine.
	// It must be: committedIndex >= appliedIndex
	appliedIndex uint64

	unstableStorage unstableStorage
	stableStorage   StableStorage
}
