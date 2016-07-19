package raft

import (
	"context"
	"testing"
	"time"

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
func Test_node_Step_unblock(t *testing.T) {
	nd := &node{
		incomingProposalMessageCh: make(chan raftpb.Message),
		doneCh: make(chan struct{}),
	}

	stopFunc := func() { close(nd.doneCh) }
	ctx, cancel := context.WithCancel(context.Background())

	tests := []struct {
		unblockFunc func()
		wErr        error
	}{
		{stopFunc, ErrStopped},
		{cancel, context.Canceled},
	}

	for i, tt := range tests {
		errc := make(chan error, 1)
		go func() {
			err := nd.Step(ctx, raftpb.Message{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER})
			errc <- err
		}()
		tt.unblockFunc()

		select {
		case err := <-errc:
			if err != tt.wErr {
				t.Fatalf("#%d: error expected %v, got %v", i, tt.wErr, err)
			}

			// clean up side-effect
			if ctx.Err() != nil {
				ctx = context.TODO()
			}

			select {
			case <-nd.doneCh:
				nd.doneCh = make(chan struct{})
			default:
			}

		case <-time.After(1 * time.Second):
			t.Fatalf("#%d: failed to unblock", i)
		}
	}
}

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
