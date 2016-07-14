package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

func Test_Step_campaign_candidate(t *testing.T) {
	rnd := newRaftNode(&Config{
		ID:         1,
		allPeerIDs: []uint64{1, 2, 3},

		ElectionTickNum:         5,
		HeartbeatTimeoutTickNum: 1,

		LeaderCheckQuorum: false,
		StorageStable:     NewStorageStableInMemory(),
		MaxEntryNumPerMsg: 0,
		MaxInflightMsgNum: 256,
		LastAppliedIndex:  0,
	})

	msg := raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		To:   1,
		From: 1,
	}
	rnd.sendToMailbox(msg)
	rnd.Step(msg)

	if rnd.state != raftpb.NODE_STATE_CANDIDATE {
		t.Fatalf("node state expected %q, got %q", raftpb.NODE_STATE_CANDIDATE, rnd.state)
	}
	if rnd.term != 1 {
		t.Fatalf("term expected 1, got %d", rnd.term)
	}
}

// (etcd raft.TestLeaderElection)
func Test_Step_leader_election(t *testing.T) {
	tests := []struct {
		fakeNetwork *fakeNetwork
		wNodeState  raftpb.NODE_STATE
	}{
		{newFakeNetwork(nil, nil, nil), raftpb.NODE_STATE_LEADER},
		{newFakeNetwork(nil, nil, noOpBlackHole), raftpb.NODE_STATE_LEADER},
		{newFakeNetwork(nil, noOpBlackHole, noOpBlackHole), raftpb.NODE_STATE_CANDIDATE},

		// quorum is 3
		{newFakeNetwork(nil, noOpBlackHole, noOpBlackHole, nil), raftpb.NODE_STATE_CANDIDATE},
		{newFakeNetwork(nil, noOpBlackHole, noOpBlackHole, nil, nil), raftpb.NODE_STATE_LEADER},

		// with higher terms than first node
		{newFakeNetwork(nil, newTestRaftNodeWithTerms(1), newTestRaftNodeWithTerms(2), newTestRaftNodeWithTerms(1, 3), nil), raftpb.NODE_STATE_FOLLOWER},

		// with higher terms including quorum, higest term in <quorum() nodes
		// out-of-range index from leader
		{newFakeNetwork(newTestRaftNodeWithTerms(1), nil, newTestRaftNodeWithTerms(2), newTestRaftNodeWithTerms(1), nil), raftpb.NODE_STATE_LEADER},
	}

	for i, tt := range tests {
		stepNode := tt.fakeNetwork.allStateMachines[1].(*raftNode)
		if stepNode.term != 0 {
			t.Fatalf("#%d: term expected 0, got %d", i, stepNode.term)
		}

		// to trigger election to 1
		tt.fakeNetwork.stepFirstFrontMessage(raftpb.Message{
			Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
			From: 1,
			To:   1,
		})

		if stepNode.state != tt.wNodeState {
			t.Fatalf("#%d: node state expected %q, got %q", i, tt.wNodeState, stepNode.state)
		}

		if stepNode.term != 1 { // should have increased
			t.Fatalf("#%d: term expected 1, got %d", i, stepNode.term)
		}
	}
}

// (etcd raft.TestLogReplication)
func Test_Step_log_replication(t *testing.T) {
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
			sm := machine.(*raftNode)

			if sm.storageRaftLog.committedIndex != tt.wCommittedIndex {
				t.Fatalf("#%d: id %x committed index expected %d, got %d", i, id, tt.wCommittedIndex, sm.storageRaftLog.committedIndex)
			}
		}
	}
}
