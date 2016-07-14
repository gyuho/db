package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestLeaderElection)
func Test_sendToMailbox_trigger_campaign(t *testing.T) {
	tests := []struct {
		fakeNetwork *fakeNetwork
		wNodeState  raftpb.NODE_STATE
	}{
		{newFakeNetwork(nil, nil, nil), raftpb.NODE_STATE_LEADER},
	}

	for i, tt := range tests {
		stepNode := tt.fakeNetwork.allStateMachines[1].(*raftNode)
		if stepNode.term != 0 {
			t.Fatalf("#%d: term expected 0, got %d", i, stepNode.term)
		}

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
func Test_sendToMailbox_MsgProp(t *testing.T) {
}
