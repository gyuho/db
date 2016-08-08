package raftsnap

import "github.com/gyuho/db/raft/raftpb"

var testSnap = &raftpb.Snapshot{
	Data: []byte("testdata"),
	Metadata: raftpb.SnapshotMetadata{
		ConfigState: raftpb.ConfigState{
			IDs: []uint64{1, 2, 3},
		},
		Index: 1,
		Term:  1,
	},
}
