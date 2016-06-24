package raft

type raftLogStorage struct {
	logger Logger

	// storage contains all stable entries since the last snapshot.
	storage Storage

	// unstableStorage contains all unstable entries and snapshot
	// to store into stableStorage.
	unstableStorage unstableStorage
}
