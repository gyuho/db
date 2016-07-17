package raft

import (
	"reflect"
	"testing"
)

// (etcd raft.TestRaftNodes)
func Test_raft_allNodeIDs(t *testing.T) {
	tests := []struct {
		ids  []uint64
		wids []uint64
	}{
		{
			[]uint64{1, 2, 3},
			[]uint64{1, 2, 3},
		},
		{
			[]uint64{3, 2, 1},
			[]uint64{1, 2, 3},
		},
	}
	for i, tt := range tests {
		rnd := newTestRaftNode(1, tt.ids, 10, 1, NewStorageStableInMemory())
		if !reflect.DeepEqual(rnd.allNodeIDs(), tt.wids) {
			t.Fatalf("#%d: all node IDs = %+v, want %+v", i, rnd.allNodeIDs(), tt.wids)
		}
	}
}
