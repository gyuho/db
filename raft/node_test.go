package raft

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestReadyContainUpdates)
func Test_node_Ready_contains_updates(t *testing.T) {
	tests := []struct {
		rd              Ready
		wContainUpdates bool
	}{
		{Ready{}, false},
		{Ready{SoftState: &raftpb.SoftState{LeaderID: 1}}, true},
		{Ready{HardStateToSave: raftpb.HardState{VotedFor: 1}}, true},
		{Ready{EntriesToSave: make([]raftpb.Entry, 1, 1)}, true},
		{Ready{EntriesToCommit: make([]raftpb.Entry, 1, 1)}, true},
		{Ready{MessagesToSend: make([]raftpb.Message, 1, 1)}, true},
		{Ready{SnapshotToSave: raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 1}}}, true},
	}

	for i, tt := range tests {
		if g := tt.rd.ContainsUpdates(); g != tt.wContainUpdates {
			t.Errorf("#%d: ContainsUpdates expected %v, got %v", i, tt.wContainUpdates, g)
		}
	}
}

// (etcd raft.TestSoftStateEqual)
func Test_node_soft_state_equal(t *testing.T) {
	tests := []struct {
		softState *raftpb.SoftState
		wEqual    bool
	}{
		{&raftpb.SoftState{}, true},
		{&raftpb.SoftState{LeaderID: 1}, false},
		{&raftpb.SoftState{NodeState: raftpb.NODE_STATE_LEADER}, false},
	}

	for i, tt := range tests {
		if g := tt.softState.Equal(&raftpb.SoftState{}); g != tt.wEqual {
			t.Fatalf("#%d: equal expected %v, got %v", i, tt.wEqual, g)
		}
	}
}

// (etcd raft.TestIsHardStateEqual)
func Test_node_hard_state_equal(t *testing.T) {
	tests := []struct {
		hardState raftpb.HardState
		wEqual    bool
	}{
		{raftpb.EmptyHardState, true},
		{raftpb.HardState{VotedFor: 1}, false},
		{raftpb.HardState{CommittedIndex: 1}, false},
		{raftpb.HardState{Term: 1}, false},
	}

	for i, tt := range tests {
		if tt.hardState.Equal(raftpb.EmptyHardState) != tt.wEqual {
			t.Errorf("#%d: equal expected %v, got %v", i, tt.wEqual, tt.hardState.Equal(raftpb.EmptyHardState))
		}
	}
}

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
func Test_node_Step_propose(t *testing.T) {
	nd := newNode()

	storage := NewStorageStableInMemory()
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, storage)
	go nd.runWithRaftNode(rnd)

	nd.Campaign(context.TODO())

	var msgs []raftpb.Message

	for {
		rd := <-nd.Ready()
		storage.Append(rd.EntriesToSave...)

		// until this becomes leader
		if rnd.id == rd.SoftState.LeaderID {
			rnd.stepFunc = func(r *raftNode, m raftpb.Message) {
				msgs = append(msgs, m)
			}
			nd.Advance()
			break
		}

		nd.Advance()
	}

	nd.Propose(context.TODO(), []byte("testdata"))
	nd.Stop()

	if len(msgs) != 1 {
		t.Fatalf("len(msgs) expected %d, got %d", len(msgs), 1)
	}
	if msgs[0].Type != raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER {
		t.Fatalf("msg.Type expected %q, got %q", raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER, msgs[0].Type)
	}
	if !bytes.Equal(msgs[0].Entries[0].Data, []byte("testdata")) {
		t.Fatalf("data expected %q, got %q", []byte("testdata"), msgs[0].Entries[0].Data)
	}
}

// (etcd raft.TestBlockProposal)
func Test_node_Step_propose_block(t *testing.T) {
	nd := newNode()
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())
	go nd.runWithRaftNode(rnd)
	defer nd.Stop()

	errc := make(chan error, 1)
	go func() {
		errc <- nd.Propose(context.TODO(), []byte("testdata"))
	}()

	time.Sleep(10 * time.Millisecond)

	select {
	case err := <-errc:
		t.Fatal(err)
	default:
	}

	nd.Campaign(context.TODO())

	select {
	case err := <-errc:
		if err != nil {
			t.Fatal(err)
		}
		// otherwise, 'nil' is returned
	case <-time.After(10 * time.Second):
		t.Fatal("blocking proposal, want unblocking")
	}
}

// (etcd raft.TestNodeTick)
func Test_node_Tick(t *testing.T) {
	nd := newNode()
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())

	go nd.runWithRaftNode(rnd)

	elapsed := rnd.electionTimeoutElapsedTickNum

	nd.Tick()

	time.Sleep(10 * time.Millisecond)

	nd.Stop()

	if rnd.electionTimeoutElapsedTickNum != elapsed+1 {
		t.Fatalf("elapsed tick expected %d, got %d", elapsed+1, rnd.electionTimeoutElapsedTickNum)
	}
}

// (etcd raft.TestNodeStop)
func Test_node_Stop(t *testing.T) {
	nd := newNode()
	rnd := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())

	donec := make(chan struct{})
	go func() {
		nd.runWithRaftNode(rnd)
		close(donec)
	}()

	elapsed := rnd.electionTimeoutElapsedTickNum

	nd.Tick()

	time.Sleep(10 * time.Millisecond)

	nd.Stop()

	select {
	case <-donec:
	case <-time.After(time.Second):
		t.Fatal("'runWithRaftNode' should have been closed after 'nd.Stop()' (took too long)")
	}

	if rnd.electionTimeoutElapsedTickNum != elapsed+1 {
		t.Fatalf("elapsed tick expected %d, got %d", elapsed+1, rnd.electionTimeoutElapsedTickNum)
	}

	// tick should have no effect
	nd.Tick()

	if rnd.electionTimeoutElapsedTickNum != elapsed+1 {
		t.Fatalf("elapsed tick expected %d, got %d", elapsed+1, rnd.electionTimeoutElapsedTickNum)
	}

	// should have no effect
	nd.Stop()
}

// (etcd raft.TestNodeStart)
func Test_node_Start(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	configChange := raftpb.ConfigChange{Type: raftpb.CONFIG_CHANGE_TYPE_ADD_NODE, NodeID: 1}
	configChangeData, err := configChange.Marshal()
	if err != nil {
		t.Fatal(err)
	}
}

// (etcd raft.TestNodeRestart)
func Test_node_restart(t *testing.T) {

}

// (etcd raft.TestNodeRestartFromSnapshot)
func Test_node_restart_from_snapshot(t *testing.T) {

}

// (etcd raft.TestNodeAdvance)
func Test_node_Advance(t *testing.T) {

}
