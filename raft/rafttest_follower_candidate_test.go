package raft

import "testing"

// (etcd raft.TestPromotable)
func Test_raft_promotableToLeader(t *testing.T) {
	tests := []struct {
		rnd         *raftNode
		wPromotable bool
	}{
		{newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory()), true},
		{newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory()), true},
		{newTestRaftNode(1, []uint64{}, 10, 1, NewStorageStableInMemory()), false},
		{newTestRaftNode(1, []uint64{2, 3}, 10, 1, NewStorageStableInMemory()), false},
	}
	for i, tt := range tests {
		if g := tt.rnd.promotableToLeader(); g != tt.wPromotable {
			t.Fatalf("#%d: promotable to leader expected %v, got %v", i, tt.wPromotable, g)
		}
	}
}

// (etcd raft.TestRecvMsgBeat)
