package raft

import (
	"bytes"
	"context"
	"math"
	"reflect"
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
		{Ready{EntriesToAppend: make([]raftpb.Entry, 1, 1)}, true},
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
		storage.Append(rd.EntriesToAppend...)

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
func Test_node_StartNode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	configChange := raftpb.ConfigChange{Type: raftpb.CONFIG_CHANGE_TYPE_ADD_NODE, NodeID: 1}
	configChangeData, err := configChange.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	st := NewStorageStableInMemory()
	nd := StartNode(&Config{
		ID:                      1,
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		StorageStable:           st,
		MaxEntryNumPerMsg:       math.MaxUint64,
		MaxInflightMsgNum:       256,
	}, []Peer{{ID: 1}})
	defer nd.Stop()

	// rd := Ready{
	// 	EntriesToAppend: rnd.storageRaftLog.unstableEntries(),
	// 	EntriesToCommit: rnd.storageRaftLog.nextEntriesToApply(),
	//
	// Ready returns a channel that receives point-in-time state of Node.
	// 'Advance' method MUST be followed, AFTER APPLYING the state in Ready.
	rd1 := <-nd.Ready()
	wrd1 := Ready{
		HardStateToSave: raftpb.HardState{CommittedIndex: 1, Term: 1, VotedFor: 0},
		EntriesToAppend: []raftpb.Entry{
			{Type: raftpb.ENTRY_TYPE_CONFIG_CHANGE, Index: 1, Term: 1, Data: configChangeData},
		},
		EntriesToCommit: []raftpb.Entry{
			{Type: raftpb.ENTRY_TYPE_CONFIG_CHANGE, Index: 1, Term: 1, Data: configChangeData},
		},
	}
	if !reflect.DeepEqual(rd1, wrd1) {
		t.Fatalf("ready expected %+v, got %+v", wrd1, rd1)
	}

	st.Append(rd1.EntriesToAppend...)
	nd.Advance()

	nd.Campaign(ctx)
	rd := <-nd.Ready()
	st.Append(rd.EntriesToAppend...)
	nd.Advance()

	nd.Propose(ctx, []byte("testdata"))

	rd2 := <-nd.Ready()
	wrd2 := Ready{
		HardStateToSave: raftpb.HardState{CommittedIndex: 3, Term: 2, VotedFor: 1},
		EntriesToAppend: []raftpb.Entry{{Index: 3, Term: 2, Data: []byte("testdata")}},
		EntriesToCommit: []raftpb.Entry{{Index: 3, Term: 2, Data: []byte("testdata")}},
	}
	if !reflect.DeepEqual(rd2, wrd2) {
		t.Fatalf("ready expected %+v, got %+v", wrd2, rd2)
	}

	st.Append(rd2.EntriesToAppend...)
	nd.Advance() // apply, commit

	select {
	case rd := <-nd.Ready():
		t.Fatalf("unexpected ready %+v", rd)
	case <-time.After(time.Millisecond):
	}
}

// (etcd raft.TestNodeAdvance)
func Test_node_StartNode_Advance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st := NewStorageStableInMemory()

	nd := StartNode(&Config{
		ID:                      1,
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		StorageStable:           st,
		MaxEntryNumPerMsg:       math.MaxUint64,
		MaxInflightMsgNum:       256,
	}, []Peer{{ID: 1}})

	defer nd.Stop()

	rd := <-nd.Ready()
	st.Append(rd.EntriesToAppend...)
	nd.Advance()

	nd.Campaign(ctx)

	rd1 := <-nd.Ready()
	wrd1 := Ready{
		SoftState:       &raftpb.SoftState{LeaderID: 1, NodeState: raftpb.NODE_STATE_LEADER},
		HardStateToSave: raftpb.HardState{CommittedIndex: 2, Term: 2, VotedFor: 1},
		EntriesToAppend: []raftpb.Entry{{Type: raftpb.ENTRY_TYPE_NORMAL, Index: 2, Term: 2}},
		EntriesToCommit: []raftpb.Entry{{Type: raftpb.ENTRY_TYPE_NORMAL, Index: 2, Term: 2}},
	}
	if !reflect.DeepEqual(rd1, wrd1) {
		t.Fatalf("ready expected %+v, got %+v", wrd1, rd1)
	}

	nd.Propose(ctx, []byte("testdata"))

	select {
	case rd = <-nd.Ready():
		t.Fatalf("unexpected ready %+v before advance", rd)
	case <-time.After(time.Millisecond):
	}

	nd.Advance()

	wrd2 := Ready{
		HardStateToSave: raftpb.HardState{CommittedIndex: 3, Term: 2, VotedFor: 1},
		EntriesToAppend: []raftpb.Entry{{Index: 3, Term: 2, Data: []byte("testdata")}},
		EntriesToCommit: []raftpb.Entry{{Index: 3, Term: 2, Data: []byte("testdata")}},
	}
	select {
	case rd2 := <-nd.Ready():
		if !reflect.DeepEqual(rd2, wrd2) {
			t.Fatalf("ready expected %+v, got %+v", wrd2, rd2)
		}

	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected ready but didn't get one")
	}
}

// (etcd raft.TestNodeRestart)
func Test_node_RestartNode(t *testing.T) {
	hardState := raftpb.HardState{CommittedIndex: 1, Term: 1}
	entriesToCommit := []raftpb.Entry{
		{Index: 1, Term: 1},
		{Index: 1, Term: 2, Data: []byte("testdata")},
	}
	wrd := Ready{
		HardStateToSave: hardState,
		EntriesToCommit: entriesToCommit[:hardState.CommittedIndex],
	}

	st := NewStorageStableInMemory()
	st.SetHardState(hardState)
	st.Append(entriesToCommit...)

	nd := RestartNode(&Config{
		ID:                      1,
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		StorageStable:           st,
		MaxEntryNumPerMsg:       math.MaxUint64,
		MaxInflightMsgNum:       256,
	})

	defer nd.Stop()

	rd := <-nd.Ready()
	if !reflect.DeepEqual(rd, wrd) {
		t.Fatalf("ready expected %+v, got %+v", wrd, rd)
	}

	nd.Advance()

	select {
	case rd := <-nd.Ready():
		t.Fatalf("unexpected ready %+v", rd)
	case <-time.After(time.Millisecond):
	}
}

// (etcd raft.TestNodeRestartFromSnapshot)
func Test_node_RestartNode_from_snapshot(t *testing.T) {
	snap := raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			ConfigState: raftpb.ConfigState{IDs: []uint64{1, 2}},
			Index:       2,
			Term:        1,
		},
	}
	entriesToCommit := []raftpb.Entry{
		{Index: 3, Term: 1, Data: []byte("testdata")},
	}
	hardState := raftpb.HardState{CommittedIndex: 3, Term: 1}

	wrd := Ready{
		HardStateToSave: hardState,
		EntriesToCommit: entriesToCommit,
	}

	st := NewStorageStableInMemory()
	st.SetHardState(hardState)
	st.ApplySnapshot(snap)
	st.Append(entriesToCommit...)

	nd := RestartNode(&Config{
		ID:                      1,
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		StorageStable:           st,
		MaxEntryNumPerMsg:       math.MaxUint64,
		MaxInflightMsgNum:       256,
	})

	defer nd.Stop()

	rd := <-nd.Ready()
	if !reflect.DeepEqual(rd, wrd) {
		t.Fatalf("ready expected %+v, got %+v", wrd, rd)
	}

	nd.Advance()

	select {
	case rd := <-nd.Ready():
		t.Fatalf("unexpected ready %+v", rd)
	case <-time.After(time.Millisecond):
	}
}
