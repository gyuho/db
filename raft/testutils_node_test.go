package raft

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

const (
	defaultTestElectionTickNum         = 10
	defaultTestHeartbeatTimeoutTickNum = 1
	defaultTestMaxEntryNumPerMsg       = math.MaxUint64
	defaultTestMaxInflightMsgNum       = 15
)

// (etcd raft.newTestRaft)
func newTestRaftNode(id uint64, allPeerIDs []uint64, electionTick, heartbeatTick int, stableStorage StorageStable) *raftNode {
	return newRaftNode(&Config{
		ID:                      id,
		allPeerIDs:              allPeerIDs,
		ElectionTickNum:         electionTick,
		HeartbeatTimeoutTickNum: heartbeatTick,
		CheckQuorum:             false,
		StorageStable:           stableStorage,
		MaxEntryNumPerMsg:       defaultTestMaxEntryNumPerMsg,
		MaxInflightMsgNum:       defaultTestMaxInflightMsgNum,
		LastAppliedIndex:        0,
	})
}

// (etcd raft.ents)
func newTestRaftNodeWithTerms(terms ...uint64) *raftNode {
	st := NewStorageStableInMemory()
	for i := range terms {
		st.Append(raftpb.Entry{Index: uint64(i + 1), Term: terms[i]})
	}

	rnd := newRaftNode(&Config{
		ID:                      1, // to be overwritten in 'newFakeNetwork'
		allPeerIDs:              nil,
		ElectionTickNum:         defaultTestElectionTickNum,
		HeartbeatTimeoutTickNum: defaultTestHeartbeatTimeoutTickNum,
		CheckQuorum:             false,
		StorageStable:           st,
		MaxEntryNumPerMsg:       defaultTestMaxEntryNumPerMsg,
		MaxInflightMsgNum:       defaultTestMaxInflightMsgNum,
		LastAppliedIndex:        0,
	})
	rnd.resetWithTerm(0)

	return rnd
}

// (etcd raft.nextEnts)
func persistALlUnstableAndApplyNextEntries(rnd *raftNode, st *StorageStableInMemory) []raftpb.Entry {
	// append all unstable entries to stable
	st.Append(rnd.storageRaftLog.unstableEntries()...)

	// lastIndex gets the last index from unstable storage first.
	// If it's not available, try to get the last index in stable storage.
	//
	// lastTerm returns the term of raftLog's last log entry.
	//
	rnd.storageRaftLog.persistedEntriesAt(rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm())

	appliedEntries := rnd.storageRaftLog.nextEntriesToApply()
	rnd.storageRaftLog.appliedTo(rnd.storageRaftLog.committedIndex)

	return appliedEntries
}

// (etcd raft.idsBySize)
func generateIDs(n int) []uint64 {
	ids := make([]uint64, n)

	for i := 0; i < n; i++ {
		ids[i] = uint64(i) + 1
	}
	return ids
}

func Test_generateIDs(t *testing.T) {
	ids := generateIDs(10)
	var prevID uint64
	for i, id := range ids {
		if i == 0 {
			prevID = id
			fmt.Printf("generated %x\n", id)
			continue
		}
		fmt.Printf("generated %x\n", id)
		if id == prevID {
			t.Fatalf("#%d: expected %x != %x", i, prevID, id)
		}

		id = prevID
	}
}

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
