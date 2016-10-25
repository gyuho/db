package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

func Test_raft_trigger_campaign_and_candidate(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	msg := raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, To: 1, From: 1}
	rnd.sendToMailbox(msg)
	rnd.Step(msg)

	rnd.assertNodeState(raftpb.NODE_STATE_CANDIDATE)
	if rnd.currentTerm != 1 {
		t.Fatalf("term expected 1, got %d", rnd.currentTerm)
	}
}

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
func Test_raft_Step_trigger_leader_heartbeat(t *testing.T) {
	tests := []struct {
		currentState raftpb.NODE_STATE
		stepFunc     func(r *raftNode, msg raftpb.Message)
		msgToStep    raftpb.Message

		wMsgNum int
	}{
		{
			raftpb.NODE_STATE_LEADER,
			stepLeader,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT, From: 1, To: 1},

			2,
		},

		{ // candidate ignores MsgBeat (MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT)
			raftpb.NODE_STATE_CANDIDATE,
			stepCandidate,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT, From: 1, To: 1},

			0,
		},

		{ // follower ignores MsgBeat (MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT)
			raftpb.NODE_STATE_FOLLOWER,
			stepFollower,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT, From: 1, To: 1},

			0,
		},
	}

	for i, tt := range tests {
		rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
		rnd.storageRaftLog = &storageRaftLog{
			storageStable: &StorageStableInMemory{snapshotEntries: []raftpb.Entry{{}, {Index: 1, Term: 0}, {Index: 2, Term: 1}}},
		}
		rnd.currentTerm = 1

		rnd.state = tt.currentState
		rnd.stepFunc = tt.stepFunc

		rnd.Step(tt.msgToStep)

		msgs := rnd.readAndClearMailbox()
		if len(msgs) != tt.wMsgNum {
			t.Fatalf("#%d: message num expected %d, got %d", i, tt.wMsgNum, len(msgs))
		}
		for _, msg := range msgs {
			if msg.Type != raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT {
				t.Fatalf("msg.Type expected %q, got %q", raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT, msg.Type)
			}
		}
	}
}

// (etcd raft.TestCampaignWhileLeader)
func Test_raft_Step_trigger_campaign_while_leader(t *testing.T) {
	for i, preVote := range []bool{false, true} {
		cfg := newTestConfig(1, []uint64{1}, 10, 1, NewStorageStableInMemory())
		cfg.PreVote = preVote
		rnd := newRaftNode(cfg)
		rnd.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

		// We don't call campaign() directly because it comes after the check
		// for our current state.
		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})
		rnd.assertNodeState(raftpb.NODE_STATE_LEADER)

		oldTerm := rnd.currentTerm

		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})
		rnd.assertNodeState(raftpb.NODE_STATE_LEADER)
		if rnd.currentTerm != oldTerm {
			t.Fatalf("#%d: term should not be increased (expected %d, got %d)", i, oldTerm, rnd.currentTerm)
		}
	}
}

// (etcd raft.TestLeaderElection)
func Test_raft_leader_election(t *testing.T) {
	for i, preVote := range []bool{false, true} {
		var cfg func(*Config)
		if preVote {
			cfg = preVoteConfig
		}

		tests := []struct {
			fakeNetwork *fakeNetwork
			wNodeState  raftpb.NODE_STATE
			expTerm     uint64
		}{
			{newFakeNetworkWithConfig(cfg, nil, nil, nil), raftpb.NODE_STATE_LEADER, 1},
			{newFakeNetworkWithConfig(cfg, nil, nil, noOpBlackHole), raftpb.NODE_STATE_LEADER, 1},
			{newFakeNetworkWithConfig(cfg, nil, noOpBlackHole, noOpBlackHole), raftpb.NODE_STATE_CANDIDATE, 1},

			{ // quorum is 3
				newFakeNetworkWithConfig(cfg, nil, noOpBlackHole, noOpBlackHole, nil),
				raftpb.NODE_STATE_CANDIDATE,
				1,
			},
			{
				newFakeNetworkWithConfig(cfg, nil, noOpBlackHole, noOpBlackHole, nil, nil),
				raftpb.NODE_STATE_LEADER,
				1,
			},

			// three logs further along than 0, but in the same term so rejections
			// are returned instead of the votes being ignored.
			{
				newFakeNetworkWithConfig(cfg, nil, newTestRaftNodeWithTerms(1), newTestRaftNodeWithTerms(1), newTestRaftNodeWithTerms(1, 1), nil), raftpb.NODE_STATE_FOLLOWER,
				1,
			},

			// logs converge
			{
				newFakeNetworkWithConfig(cfg, newTestRaftNodeWithTerms(1), nil, newTestRaftNodeWithTerms(2), newTestRaftNodeWithTerms(1), nil),
				raftpb.NODE_STATE_LEADER,
				2,
			},
		}

		for j, tt := range tests {
			// to trigger election to 1
			tt.fakeNetwork.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})

			stepNode := tt.fakeNetwork.allStateMachines[1].(*raftNode)

			var expState raftpb.NODE_STATE
			var expTerm uint64

			if tt.wNodeState == raftpb.NODE_STATE_CANDIDATE && preVote {
				// In pre-vote mode, an election that fails to complete
				// leaves the node in pre-candidate state without advancing the term.
				expState = raftpb.NODE_STATE_PRE_CANDIDATE
				expTerm = 0
			} else {
				expState = tt.wNodeState
				expTerm = tt.expTerm
			}

			if stepNode.state != expState {
				t.Fatalf("#%d-%d: node state expected %q, got %q", i, j, expState, stepNode.state)
			}
			if stepNode.currentTerm != expTerm {
				t.Fatalf("#%d-%d: term expected %d, got %d", i, j, expTerm, stepNode.currentTerm)
			}
		}
	}
}

// (etcd raft.TestRecvMsgVote)
func Test_raft_receive_msg_vote(t *testing.T) {
	for i, msgType := range []raftpb.MESSAGE_TYPE{raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE, raftpb.MESSAGE_TYPE_PRE_CANDIDATE_REQUEST_VOTE} {
		tests := []struct {
			state   raftpb.NODE_STATE
			i, term uint64
			voteFor uint64
			wreject bool
		}{
			{raftpb.NODE_STATE_FOLLOWER, 0, 0, NoNodeID, true},
			{raftpb.NODE_STATE_FOLLOWER, 0, 1, NoNodeID, true},
			{raftpb.NODE_STATE_FOLLOWER, 0, 2, NoNodeID, true},
			{raftpb.NODE_STATE_FOLLOWER, 0, 3, NoNodeID, false},

			{raftpb.NODE_STATE_FOLLOWER, 1, 0, NoNodeID, true},
			{raftpb.NODE_STATE_FOLLOWER, 1, 1, NoNodeID, true},
			{raftpb.NODE_STATE_FOLLOWER, 1, 2, NoNodeID, true},
			{raftpb.NODE_STATE_FOLLOWER, 1, 3, NoNodeID, false},

			{raftpb.NODE_STATE_FOLLOWER, 2, 0, NoNodeID, true},
			{raftpb.NODE_STATE_FOLLOWER, 2, 1, NoNodeID, true},
			{raftpb.NODE_STATE_FOLLOWER, 2, 2, NoNodeID, false},
			{raftpb.NODE_STATE_FOLLOWER, 2, 3, NoNodeID, false},

			{raftpb.NODE_STATE_FOLLOWER, 3, 0, NoNodeID, true},
			{raftpb.NODE_STATE_FOLLOWER, 3, 1, NoNodeID, true},
			{raftpb.NODE_STATE_FOLLOWER, 3, 2, NoNodeID, false},
			{raftpb.NODE_STATE_FOLLOWER, 3, 3, NoNodeID, false},

			{raftpb.NODE_STATE_FOLLOWER, 3, 2, 2, false},
			{raftpb.NODE_STATE_FOLLOWER, 3, 2, 1, true},

			{raftpb.NODE_STATE_LEADER, 3, 3, 1, true},
			{raftpb.NODE_STATE_PRE_CANDIDATE, 3, 3, 1, true},
			{raftpb.NODE_STATE_CANDIDATE, 3, 3, 1, true},
		}

		for j, tt := range tests {
			sm := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())
			sm.state = tt.state
			switch tt.state {
			case raftpb.NODE_STATE_FOLLOWER:
				sm.stepFunc = stepFollower
			case raftpb.NODE_STATE_CANDIDATE, raftpb.NODE_STATE_PRE_CANDIDATE:
				sm.stepFunc = stepCandidate
			case raftpb.NODE_STATE_LEADER:
				sm.stepFunc = stepLeader
			}
			sm.votedFor = tt.voteFor
			sm.storageRaftLog = &storageRaftLog{
				storageStable:   &StorageStableInMemory{snapshotEntries: []raftpb.Entry{{}, {Index: 1, Term: 2}, {Index: 2, Term: 2}}},
				storageUnstable: storageUnstable{indexOffset: 3},
			}

			sm.Step(raftpb.Message{Type: msgType, From: 2, LogIndex: tt.i, LogTerm: tt.term})

			msgs := sm.readAndClearMailbox()
			if g := len(msgs); g != 1 {
				t.Fatalf("#%d-%d: len(msgs) = %d, want 1", i, j, g)
				continue
			}
			if g := msgs[0].Type; g != raftpb.VoteResponseType(msgType) {
				t.Fatalf("#%d-%d: m.Type = %v, want %v", i, j, g, raftpb.VoteResponseType(msgType))
			}
			if g := msgs[0].Reject; g != tt.wreject {
				t.Fatalf("#%d-%d: m.Reject = %v, want %v", i, j, g, tt.wreject)
			}
		}
	}
}

// (etcd raft.TestStateTransition)
func Test_raft_node_state_transition(t *testing.T) {
	tests := []struct {
		from   raftpb.NODE_STATE
		to     raftpb.NODE_STATE
		wallow bool
		wterm  uint64
		wlead  uint64
	}{
		{raftpb.NODE_STATE_FOLLOWER, raftpb.NODE_STATE_FOLLOWER, true, 1, NoNodeID},
		{raftpb.NODE_STATE_FOLLOWER, raftpb.NODE_STATE_PRE_CANDIDATE, true, 0, NoNodeID},
		{raftpb.NODE_STATE_FOLLOWER, raftpb.NODE_STATE_CANDIDATE, true, 1, NoNodeID},
		{raftpb.NODE_STATE_FOLLOWER, raftpb.NODE_STATE_LEADER, false, 0, NoNodeID},

		{raftpb.NODE_STATE_PRE_CANDIDATE, raftpb.NODE_STATE_FOLLOWER, true, 0, NoNodeID},
		{raftpb.NODE_STATE_PRE_CANDIDATE, raftpb.NODE_STATE_PRE_CANDIDATE, true, 0, NoNodeID},
		{raftpb.NODE_STATE_PRE_CANDIDATE, raftpb.NODE_STATE_CANDIDATE, true, 1, NoNodeID},
		{raftpb.NODE_STATE_PRE_CANDIDATE, raftpb.NODE_STATE_LEADER, true, 0, 1},

		{raftpb.NODE_STATE_CANDIDATE, raftpb.NODE_STATE_FOLLOWER, true, 0, NoNodeID},
		{raftpb.NODE_STATE_CANDIDATE, raftpb.NODE_STATE_PRE_CANDIDATE, true, 0, NoNodeID},
		{raftpb.NODE_STATE_CANDIDATE, raftpb.NODE_STATE_CANDIDATE, true, 1, NoNodeID},
		{raftpb.NODE_STATE_CANDIDATE, raftpb.NODE_STATE_LEADER, true, 0, 1},

		{raftpb.NODE_STATE_LEADER, raftpb.NODE_STATE_FOLLOWER, true, 1, NoNodeID},
		{raftpb.NODE_STATE_LEADER, raftpb.NODE_STATE_PRE_CANDIDATE, false, 0, NoNodeID},
		{raftpb.NODE_STATE_LEADER, raftpb.NODE_STATE_CANDIDATE, false, 1, NoNodeID},
		{raftpb.NODE_STATE_LEADER, raftpb.NODE_STATE_LEADER, true, 0, 1},
	}

	for i, tt := range tests {
		func() {
			defer func() {
				if r := recover(); r != nil {
					if tt.wallow {
						t.Fatalf("%d: allow = %v, want %v", i, false, true)
					}
				}
			}()

			sm := newTestRaftNode(1, []uint64{1}, 10, 1, NewStorageStableInMemory())
			sm.state = tt.from

			switch tt.to {
			case raftpb.NODE_STATE_FOLLOWER:
				sm.becomeFollower(tt.wterm, tt.wlead)
			case raftpb.NODE_STATE_PRE_CANDIDATE:
				sm.becomePreCandidate()
			case raftpb.NODE_STATE_CANDIDATE:
				sm.becomeCandidate()
			case raftpb.NODE_STATE_LEADER:
				sm.becomeLeader()
			}

			if sm.currentTerm != tt.wterm {
				t.Fatalf("%d: term = %d, want %d", i, sm.currentTerm, tt.wterm)
			}
			if sm.leaderID != tt.wlead {
				t.Fatalf("%d: leaderID = %d, want %d", i, sm.leaderID, tt.wlead)
			}
		}()
	}
}

// (etcd raft.TestLeaderCycle)
// TestLeaderCycle verifies that each node in a cluster can campaign
// and be elected in turn. This ensures that elections (including
// pre-vote) work when not starting from a clean slate (as they do in
// TestLeaderElection)
func Test_raft_leader_cycle(t *testing.T) {
	for _, preVote := range []bool{false, true} {
		var cfg func(*Config)
		if preVote {
			cfg = preVoteConfig
		}
		n := newFakeNetworkWithConfig(cfg, nil, nil, nil)
		for campaignerID := uint64(1); campaignerID <= 3; campaignerID++ {
			n.stepFirstMessage(raftpb.Message{From: campaignerID, To: campaignerID, Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN})

			for _, peer := range n.allStateMachines {
				sm := peer.(*raftNode)
				if sm.id == campaignerID && sm.state != raftpb.NODE_STATE_LEADER {
					t.Fatalf("preVote=%v: campaigning node %d state = %v, want raftpb.NODE_STATE_LEADER",
						preVote, sm.id, sm.state)
				} else if sm.id != campaignerID && sm.state != raftpb.NODE_STATE_FOLLOWER {
					t.Fatalf("preVote=%v: after campaign of node %d, "+
						"node %d had state = %v, want raftpb.NODE_STATE_FOLLOWER",
						preVote, campaignerID, sm.id, sm.state)
				}
			}
		}
	}
}

// (etcd raft.TestVoteFromAnyState)
func Test_raft_vote_from_any_node_state(t *testing.T) {
	for _, vt := range []raftpb.MESSAGE_TYPE{raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE, raftpb.MESSAGE_TYPE_PRE_CANDIDATE_REQUEST_VOTE} {
		for st := raftpb.NODE_STATE(0); st < raftpb.NODE_STATE_NUM_NODE_STATE; st++ {
			r := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
			r.currentTerm = 1

			switch st {
			case raftpb.NODE_STATE_FOLLOWER:
				r.becomeFollower(r.currentTerm, 3)
			case raftpb.NODE_STATE_PRE_CANDIDATE:
				r.becomePreCandidate()
			case raftpb.NODE_STATE_CANDIDATE:
				r.becomeCandidate()
			case raftpb.NODE_STATE_LEADER:
				r.becomeCandidate()
				r.becomeLeader()
			}

			// Note that setting our state above may have advanced r.currentTerm
			// past its initial value.
			origTerm := r.currentTerm
			newTerm := r.currentTerm + 1

			msg := raftpb.Message{
				From:              2,
				To:                1,
				Type:              vt,
				SenderCurrentTerm: newTerm,
				LogTerm:           newTerm,
				LogIndex:          42,
			}
			if err := r.Step(msg); err != nil {
				t.Fatalf("%s,%s: Step failed: %s", vt, st, err)
			}
			if len(r.mailbox) != 1 {
				t.Fatalf("%s,%s: %d response messages, want 1: %+v", vt, st, len(r.mailbox), r.mailbox)
			} else {
				resp := r.mailbox[0]
				if resp.Type != raftpb.VoteResponseType(vt) {
					t.Fatalf("%s,%s: response message is %s, want %s", vt, st, resp.Type, raftpb.VoteResponseType(vt))
				}
				if resp.Reject {
					t.Fatalf("%s,%s: unexpected rejection", vt, st)
				}
			}

			// If this was a real vote, we reset our state and term.
			if vt == raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE {
				if r.state != raftpb.NODE_STATE_FOLLOWER {
					t.Fatalf("%s,%s: state %s, want %s", vt, raftpb.NODE_STATE_FOLLOWER, r.state, st)
				}
				if r.currentTerm != newTerm {
					t.Fatalf("%s,%s: term %d, want %d", vt, st, r.currentTerm, newTerm)
				}
				if r.votedFor != 2 {
					t.Fatalf("%s,%s: votedFor %d, want 2", vt, st, r.votedFor)
				}
			} else {
				// In a prevote, nothing changes.
				if r.state != st {
					t.Fatalf("%s,%s: state %s, want %s", vt, st, r.state, st)
				}
				if r.currentTerm != origTerm {
					t.Fatalf("%s,%s: term %d, want %d", vt, st, r.currentTerm, origTerm)
				}
				// if st == raftpb.NODE_STATE_FOLLOWER  or StatePreCandidate, r hasn't voted yet.
				// In StateCandidate or StateLeader, it's voted for itself.
				if r.votedFor != NoNodeID && r.votedFor != 1 {
					t.Fatalf("%s,%s: votedFor %d, want %d or 1", vt, st, r.votedFor, NoNodeID)
				}
			}
		}
	}
}

// (etcd raft.TestSingleNodeCandidate)
func Test_raft_leader_election_single_node(t *testing.T) {
	fn := newFakeNetwork(nil)

	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})

	rnd1 := fn.allStateMachines[1].(*raftNode)
	if rnd1.state != raftpb.NODE_STATE_LEADER {
		t.Fatalf("rnd1 state expected %q, got %q", raftpb.NODE_STATE_LEADER, rnd1.state)
	}
}

// (etcd raft.TestSingleNodePreCandidate)
func Test_raft_single_node_pre_candidate(t *testing.T) {
	tt := newFakeNetworkWithConfig(preVoteConfig, nil)
	tt.stepFirstMessage(raftpb.Message{From: 1, To: 1, Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN})

	rnd := tt.allStateMachines[1].(*raftNode)
	if rnd.state != raftpb.NODE_STATE_LEADER {
		t.Fatalf("rnd state expected %q, got %q", raftpb.NODE_STATE_LEADER, rnd.state)
	}
}

// (etcd raft.TestCandidateConcede)
func Test_raft_candidate_concede(t *testing.T) {

}

// (etcd raft.TestDuelingPreCandidates)
func Test_raft_dueling_pre_candidate(t *testing.T) {

}

// (etcd raft.TestAllServerStepdown)
func Test_raft_all_server_step_down(t *testing.T) {

}

// (etcd raft.TestLeaderStepdownWhenQuorumActive)
func Test_raft_leader_step_down_checkQuorum_active(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 5, 1, NewStorageStableInMemory())
	rnd.checkQuorum = true

	rnd.becomeCandidate()
	rnd.becomeLeader()

	for i := 0; i < rnd.electionTimeoutTickNum+1; i++ {
		rnd.Step(raftpb.Message{From: 2, Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT, SenderCurrentTerm: rnd.currentTerm})
		rnd.tickFunc()
	}

	if rnd.state != raftpb.NODE_STATE_LEADER {
		t.Fatalf("state = %v, want %v", rnd.state, raftpb.NODE_STATE_LEADER)
	}
}

// (etcd raft.TestLeaderStepdownWhenQuorumLost)
func Test_raft_leader_step_down_checkQuorum_lost(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 5, 1, NewStorageStableInMemory())
	rnd.checkQuorum = true

	rnd.becomeCandidate()
	rnd.becomeLeader()

	for i := 0; i < rnd.electionTimeoutTickNum+1; i++ {
		rnd.tickFunc()
	}

	if rnd.state != raftpb.NODE_STATE_FOLLOWER {
		t.Fatalf("state = %v, want %v", rnd.state, raftpb.NODE_STATE_FOLLOWER)
	}
}

func Test_raft_leader_election_checkQuorum_candidate(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd3 := newTestRaftNode(3, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd1.checkQuorum = true
	rnd2.checkQuorum = true
	rnd3.checkQuorum = true

	fn := newFakeNetwork(rnd1, rnd2, rnd3)

	// (Raft §4.2.3 Disruptive servers, p.42)
	//
	// "if a server receives a RequestVote request within the minimum election timeout
	// of hearing from a current leader, it does not update its term or grant its vote."
	//
	// this helps avoid disruptions from servers with old configuration
	//
	// If checkQuorum is true, leader checks if quorum of cluster are active for every election timeout
	// (if rnd.allProgresses[id].RecentActive {activeN++}).
	// And leader maintains 'Progress.RecentActive' for every incoming message from follower.
	// Now, if quorum is not active, leader reverts back to follower.
	//
	// Leader sends internal check-quorum message to trigger quorum-check
	// for every election timeout (raftNode.tickFuncLeaderHeartbeatTimeout).
	//
	// So if checkQuorum is true and election timeout has not happened yet,
	// then leader is guaranteed to have been in contact with quorum within
	// the last election timeout, as a valid leader.
	// So it shouldn't increase its term.
	//
	// SO, it's ok to to reject vote request

	// for i := 0; i < rnd1.electionTimeoutTickNum; i++ {
	// 	rnd1.tickFunc()
	// }

	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})

	if rnd1.state != raftpb.NODE_STATE_LEADER {
		t.Fatalf("rnd1 state expected %q, got %q", raftpb.NODE_STATE_LEADER, rnd1.state)
	}
}

// (etcd raft.TestLeaderElectionWithCheckQuorum)
func Test_raft_leader_election_checkQuorum_leader(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd3 := newTestRaftNode(3, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd1.checkQuorum = true
	rnd2.checkQuorum = true
	rnd3.checkQuorum = true

	fn := newFakeNetwork(rnd1, rnd2, rnd3)

	// need to reset randomizedElectionTimeout larger than electionTimeout again,
	// because the value might be reset to electionTimeout since the last state changes
	rnd1.setRandomizedElectionTimeoutTickNum(rnd1.electionTimeoutTickNum + 1)
	rnd2.setRandomizedElectionTimeoutTickNum(rnd2.electionTimeoutTickNum + 2)

	// time out rnd2, so that rnd2 now does not ignore the vote request
	// and rnd2 becomes the follower. And rnd1 will start campaign, and rnd2
	// will be able to vote for rnd1 to be a leader
	// for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
	// 	rnd2.tickFunc()
	// }

	// Immediately after creation, votes are cast regardless of the
	// election timeout.

	// rnd1 will start campaign and become the leader
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

	// rnd1 check quorum is true, and passed election timeout
	// so it won't ignore the vote request, and then revert back to follower
	for i := 0; i < rnd1.electionTimeoutTickNum; i++ {
		rnd1.tickFunc()
	}

	// rnd2 check quorum is true, and passed election timeout
	// so it won't ignore the vote request, and then revert back to follower
	for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
		rnd2.tickFunc()
	}

	// rnd3 will start campaign and become the leader
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 3, To: 3})
	rnd1.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_LEADER)
}

// (etcd raft.TestLeaderSupersedingWithCheckQuorum)
func Test_raft_leader_election_checkQuorum_leader_ignore_vote(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd3 := newTestRaftNode(3, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd1.checkQuorum = true
	rnd2.checkQuorum = true
	rnd3.checkQuorum = true

	fn := newFakeNetwork(rnd1, rnd2, rnd3)

	// func (rnd *raftNode) pastElectionTimeout() bool {
	// 	return rnd.electionTimeoutElapsedTickNum >= rnd.randomizedElectionTimeoutTickNum
	// }
	rnd2.setRandomizedElectionTimeoutTickNum(rnd2.electionTimeoutTickNum + 1) // to prevent rnd2 from election timeout

	for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
		rnd2.tickFunc()
	}

	// rnd1 will start campaign and become the leader
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

	// rnd3 will start campaign
	// but the vote requests will be rejected by rnd2
	// not yet timing out its election timeout
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 3, To: 3})

	// current leader rnd1 should ignore the vote request from rnd2
	// because it's leader
	//
	// 1. not leader transfer
	// 2. not candidate
	// 3. checkQuorum is true
	//    election timeout has not happened
	//
	if rnd3.state != raftpb.NODE_STATE_CANDIDATE {
		t.Fatalf("rnd3 state expected %q, got %q", raftpb.NODE_STATE_CANDIDATE, rnd3.state)
	}

	// election timeout
	for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
		rnd2.tickFunc()
	}

	// rnd3 will start campaign and become the leader
	fn.stepFirstMessage(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
		From: 3,
		To:   3,
	})

	if rnd3.state != raftpb.NODE_STATE_LEADER {
		t.Fatalf("rnd3 state expected %q, got %q", raftpb.NODE_STATE_LEADER, rnd3.state)
	}
}

// (etcd raft.TestFreeStuckCandidateWithCheckQuorum)
func Test_raft_leader_election_checkQuorum_candidate_with_higher_term(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd3 := newTestRaftNode(3, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd1.checkQuorum = true
	rnd2.checkQuorum = true
	rnd3.checkQuorum = true

	fn := newFakeNetwork(rnd1, rnd2, rnd3)
	rnd2.setRandomizedElectionTimeoutTickNum(rnd2.electionTimeoutTickNum + 1)

	// time out rnd2, so that rnd2 now does not ignore the vote request
	// and rnd2 becomes the follower. And rnd1 will start campaign, and rnd2
	// will be able to vote for rnd1 to be a leader
	// for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
	// 	rnd2.tickFunc()
	// }

	// rnd1 will start campaign and become the leader
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

	// isolate leader (cut all outgoing, incoming connections)
	fn.isolate(1)

	// trigger election from 3
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 3, To: 3})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_CANDIDATE)
	if rnd2.currentTerm+1 != rnd3.currentTerm {
		t.Fatalf("rnd2 expected term increase %d, got %d", rnd2.currentTerm+1, rnd3.currentTerm)
	}

	// trigger election from 3, again for safety
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 3, To: 3})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_CANDIDATE)
	if rnd2.currentTerm+2 != rnd3.currentTerm {
		t.Fatalf("rnd2 expected term increase %d, got %d", rnd2.currentTerm+2, rnd3.currentTerm)
	}

	// recover all dropped connections
	fn.recoverAll()

	// was-isolated leader now sends heartbeat to now-new-candidate
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT, From: 1, To: 3})

	// once candidate rnd3 gets the heartbeat from leader,
	// it must revert back to follower first
	//
	// And rnd3 handles heartbeat and responds with higher term
	// and then leader rnd1 gets the response with higher term,
	// so it reverts back to follower
	rnd1.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	rnd3.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	if rnd3.currentTerm != rnd1.currentTerm {
		t.Fatalf("rnd2 term expected %d, got %d", rnd1.currentTerm, rnd3.currentTerm)
	}
}

// (etcd raft.TestNonPromotableVoterWithCheckQuorum)
func Test_raft_leader_election_checkQuorum_non_promotable_voter(t *testing.T) {
	rnd1 := newTestRaftNode(1, []uint64{1, 2}, 10, 1, NewStorageStableInMemory())
	rnd2 := newTestRaftNode(2, []uint64{1}, 10, 1, NewStorageStableInMemory())

	rnd1.checkQuorum = true
	rnd2.checkQuorum = true

	fn := newFakeNetwork(rnd1, rnd2)
	rnd2.setRandomizedElectionTimeoutTickNum(rnd2.electionTimeoutTickNum + 1)

	// remove again, because it is updated inside 'newFakeNetwork'
	rnd2.deleteProgress(2)
	if rnd2.promotableToLeader() { // _, ok := rnd.allProgresses[rnd.id]
		t.Fatalf("rnd2 must not be promotable %s", rnd2.describe())
	}

	// time out rnd2
	for i := 0; i < rnd2.electionTimeoutTickNum; i++ {
		rnd2.tickFunc()
	}

	// trigger campaign in rnd1
	fn.stepFirstMessage(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})
	rnd1.assertNodeState(raftpb.NODE_STATE_LEADER)
	rnd2.assertNodeState(raftpb.NODE_STATE_FOLLOWER)
	if rnd1.leaderID != 1 {
		t.Fatalf("leader must be 1, got %d", rnd1.leaderID)
	}
	if rnd2.leaderID != 1 {
		t.Fatalf("leader must be 1, got %d", rnd2.leaderID)
	}
}
