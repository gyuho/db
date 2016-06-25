package raft

type raftLogStorage struct {
	logger Logger

	// storage contains all stable entries since the last snapshot.
	storage Storage

	// storageUnstable contains all unstable entries and snapshot
	// to store into stableStorage.
	storageUnstable storageUnstable
}
