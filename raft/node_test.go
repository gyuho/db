package raft

import (
	"context"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestNodeStep)
func Test_node_Step(t *testing.T) {
	for i := range raftpb.MESSAGE_TYPE_name {
		msgType := raftpb.MESSAGE_TYPE(i)

		nd := &node{
			incomingProposalMessageCh: make(chan raftpb.Message, 1),
			incomingMessageCh:         make(chan raftpb.Message, 1),
		}
		nd.Step(context.TODO(), raftpb.Message{Type: msgType})

		// only proposal goes to proposal channel
		switch msgType {
		case raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER:
			select {
			case <-nd.incomingProposalMessageCh:
			default:
				t.Fatalf("received unexpected message %q from incomingProposalMessageCh", msgType)
			}

		default:
			select {
			case <-nd.incomingMessageCh:
				if raftpb.IsInternalMessage(msgType) {
					t.Fatalf("internal message %q SHOULD NOT BE passed to incomingMessageCh", msgType)
				}
			default:
				if !raftpb.IsInternalMessage(msgType) {
					t.Fatalf("non-internal message %q SHOULD BE passed to incomingMessageCh", msgType)
				}
			}
		}
	}
}

// (etcd raft.TestNodeStepUnblock)

// (etcd raft.TestNodePropose)

// (etcd raft.TestNodeReadIndex)

// (etcd raft.TestNodeProposeConfig)

// (etcd raft.TestBlockProposal)

// (etcd raft.TestNodeTick)

// (etcd raft.TestNodeStop)

// (etcd raft.TestReadyContainUpdates)

// (etcd raft.TestNodeStart)

// (etcd raft.TestNodeRestart)

// (etcd raft.TestNodeRestartFromSnapshot)

// (etcd raft.TestNodeAdvance)

// (etcd raft.TestSoftStateEqual)

// (etcd raft.TestIsHardStateEqual)
