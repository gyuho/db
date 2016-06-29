package raft

type raftLogStorage struct {
	// storageStable contains all stable entries since the last snapshot.
	storageStable StorageStable

	// storageUnstable contains all unstable entries and snapshot to store into stableStorage.
	storageUnstable storageUnstable
}
