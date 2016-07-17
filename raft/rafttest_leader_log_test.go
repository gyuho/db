package raft

import (
	"bytes"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestLogReplication)
func Test_raft_log_replication(t *testing.T) {
	tests := []struct {
		fakeNetwork        *fakeNetwork
		msgsToSendOneByOne []raftpb.Message

		wCommittedIndex uint64
	}{
		{
			newFakeNetwork(nil, nil, nil),
			[]raftpb.Message{
				{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, From: 1, To: 1, Entries: []raftpb.Entry{{Data: []byte("testdata")}}},
			},

			2,
		},

		{
			newFakeNetwork(nil, nil, nil),
			[]raftpb.Message{
				{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, From: 1, To: 1, Entries: []raftpb.Entry{{Data: []byte("testdata")}}},
				{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 2},
				{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, From: 1, To: 2, Entries: []raftpb.Entry{{Data: []byte("testdata")}}},
			},

			4,
		},
	}

	for i, tt := range tests {
		// to trigger election to 1
		tt.fakeNetwork.stepFirstFrontMessage(raftpb.Message{
			Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
			From: 1,
			To:   1,
		})

		for _, m := range tt.msgsToSendOneByOne {
			tt.fakeNetwork.stepFirstFrontMessage(m)
		}

		for id, machine := range tt.fakeNetwork.allStateMachines {
			rnd := machine.(*raftNode)

			if rnd.storageRaftLog.committedIndex != tt.wCommittedIndex {
				t.Fatalf("#%d: id %x committed index expected %d, got %d", i, id, tt.wCommittedIndex, rnd.storageRaftLog.committedIndex)
			}

			var appliedEntries []raftpb.Entry
			for _, ent := range persistALlUnstableAndApplyNextEntries(rnd, tt.fakeNetwork.allStableStorageInMemory[id]) {
				if ent.Data != nil {
					appliedEntries = append(appliedEntries, ent)
				}
			}

			var proposedEntries []raftpb.Message
			for _, m := range tt.msgsToSendOneByOne {
				if m.Type == raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER {
					proposedEntries = append(proposedEntries, m)
				}
			}

			// ensure all proposed entries are applied
			for k := range proposedEntries {
				ae := appliedEntries[k]
				pe := proposedEntries[k]
				if !bytes.Equal(ae.Data, pe.Entries[0].Data) {
					t.Fatalf("#%d.%d: id %x entry data expected %q, got %q", i, k, id, pe.Entries[0].Data, ae.Data)
				}
			}
		}
	}
}
