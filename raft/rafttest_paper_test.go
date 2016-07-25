package raft

import (
	"reflect"
	"sort"
	"testing"

	"github.com/gyuho/db/pkg/xlog"
	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.TestFollowerUpdateTermFromMessage)
func Test_raft_paper_follower_update_term_from_Message(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd.becomeFollower(1, 2) // leaderID 2

	// case msg.SenderCurrentTerm > rnd.currentTerm:
	//
	// func (rnd *raftNode) sendToMailbox(msg raftpb.Message) {
	//   msg.From = rnd.id
	//
	rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, SenderCurrentTerm: 2})

	if rnd.currentTerm != 2 {
		t.Fatalf("term expected 2, got %d", rnd.currentTerm)
	}
	if rnd.state != raftpb.NODE_STATE_FOLLOWER {
		t.Fatalf("node state expected %q, got %q", raftpb.NODE_STATE_FOLLOWER, rnd.state)
	}
}

// (etcd raft.TestCandidateUpdateTermFromMessage)
func Test_raft_paper_candidate_update_term_from_Message(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd.becomeCandidate()

	// case msg.SenderCurrentTerm > rnd.currentTerm:
	//
	// func (rnd *raftNode) sendToMailbox(msg raftpb.Message) {
	//   msg.From = rnd.id
	//
	rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, SenderCurrentTerm: 2})

	if rnd.currentTerm != 2 {
		t.Fatalf("term expected 2, got %d", rnd.currentTerm)
	}
	if rnd.state != raftpb.NODE_STATE_FOLLOWER {
		t.Fatalf("node state expected %q, got %q", raftpb.NODE_STATE_FOLLOWER, rnd.state)
	}
}

// (etcd raft.TestLeaderUpdateTermFromMessage)
func Test_raft_paper_leader_update_term_from_Message(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd.becomeCandidate()
	rnd.becomeLeader()

	// case msg.SenderCurrentTerm > rnd.currentTerm:
	//
	// func (rnd *raftNode) sendToMailbox(msg raftpb.Message) {
	//   msg.From = rnd.id
	//
	rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, SenderCurrentTerm: 2})

	if rnd.currentTerm != 2 {
		t.Fatalf("term expected 2, got %d", rnd.currentTerm)
	}
	if rnd.state != raftpb.NODE_STATE_FOLLOWER {
		t.Fatalf("node state expected %q, got %q", raftpb.NODE_STATE_FOLLOWER, rnd.state)
	}
}

// (etcd raft.TestStartAsFollower)
//
// (Raft §3.3 Raft basics, p.14)
func Test_raft_paper_start_as_follower(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	if rnd.state != raftpb.NODE_STATE_FOLLOWER {
		t.Fatalf("state expected %q, got %q", raftpb.NODE_STATE_FOLLOWER, rnd.state)
	}
}

// (etcd raft.TestRejectStaleTermMessage)
//
// (Raft §3.3 Raft basics, p.15)
func Test_raft_paper_reject_stale_term_message(t *testing.T) {
	called := false
	stepFuncTest := func(rnd *raftNode, msg raftpb.Message) {
		called = true
	}

	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd.stepFunc = stepFuncTest
	rnd.loadHardState(raftpb.HardState{Term: 2})

	rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, SenderCurrentTerm: rnd.currentTerm - 1})

	if called {
		t.Fatal("message should have been rejected not calling stepFunc")
	}
}

// (etcd raft.TestLeaderBcastBeat)
func Test_raft_paper_leader_broadcast_heartbeat(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory()) // heartbeat tick is 1
	rnd.becomeCandidate()
	rnd.becomeLeader()

	for i := 0; i < 10; i++ {
		rnd.leaderAppendEntriesToLeader(raftpb.Entry{Index: uint64(i) + 1})
	}

	// to trigger leader to send heartbeat
	for i := 0; i < rnd.heartbeatTimeoutTickNum; i++ {
		rnd.tickFunc()
	}
	// tickFunc
	//
	// if rnd.heartbeatTimeoutElapsedTickNum >= rnd.heartbeatTimeoutTickNum {
	// 	rnd.heartbeatTimeoutElapsedTickNum = 0
	// 	rnd.Step(raftpb.Message{
	// 		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT,
	// 		From: rnd.id,
	// 	})
	// }

	msgs := rnd.readAndClearMailbox()
	sort.Sort(messageSlice(msgs))

	wmsgs := []raftpb.Message{
		{Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT, From: 1, To: 2, SenderCurrentTerm: 1},
		{Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT, From: 1, To: 3, SenderCurrentTerm: 1},
	}
	if !reflect.DeepEqual(msgs, wmsgs) {
		t.Fatalf("messages expected %+v, got %+v", wmsgs, msgs)
	}
}

// (Raft §3.4 Leader election, p.16)
// (etcd raft.TestFollowerStartElection)
func Test_raft_paper_follower_start_election(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd.becomeFollower(1, 2) // term 1, leader is 2
	oldTerm := rnd.currentTerm

	// election timeout to trigger election from follower
	for i := 0; i < 2*rnd.electionTimeoutTickNum; i++ {
		rnd.tickFunc()
	}

	if rnd.currentTerm != oldTerm+1 {
		t.Fatalf("term should have increased to %d, got %d", oldTerm+1, rnd.currentTerm)
	}

	rnd.assertNodeState(raftpb.NODE_STATE_CANDIDATE)

	if !rnd.votedFrom[rnd.id] {
		t.Fatalf("should have voted for itself, got %+v", rnd.votedFrom)
	}

	msgs := rnd.readAndClearMailbox()
	sort.Sort(messageSlice(msgs))

	// follower is now candidate sending vote-requests
	wmsgs := []raftpb.Message{
		{Type: raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE, From: 1, To: 2, SenderCurrentTerm: 2},
		{Type: raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE, From: 1, To: 3, SenderCurrentTerm: 2},
	}
	if !reflect.DeepEqual(msgs, wmsgs) {
		t.Fatalf("messages expected %+v, got %+v", wmsgs, msgs)
	}
}

// (Raft §3.4 Leader election, p.16)
// (etcd raft.TestCandidateStartNewElection)
func Test_raft_paper_candidate_start_election(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	rnd.becomeCandidate() // term 2
	oldTerm := rnd.currentTerm

	// election timeout to trigger another election from candidate
	// candidate starts a new election and increments its term
	for i := 0; i < 2*rnd.electionTimeoutTickNum; i++ {
		rnd.tickFunc()
	}

	if rnd.currentTerm != oldTerm+1 {
		t.Fatalf("term should have increased to %d, got %d", oldTerm+1, rnd.currentTerm)
	}

	rnd.assertNodeState(raftpb.NODE_STATE_CANDIDATE)

	if !rnd.votedFrom[rnd.id] {
		t.Fatalf("should have voted for itself, got %+v", rnd.votedFrom)
	}

	msgs := rnd.readAndClearMailbox()
	sort.Sort(messageSlice(msgs))

	// follower is now candidate sending vote-requests
	wmsgs := []raftpb.Message{
		{Type: raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE, From: 1, To: 2, SenderCurrentTerm: 2},
		{Type: raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE, From: 1, To: 3, SenderCurrentTerm: 2},
	}
	if !reflect.DeepEqual(msgs, wmsgs) {
		t.Fatalf("messages expected %+v, got %+v", wmsgs, msgs)
	}
}

// (etcd raft.TestLeaderElectionInOneRoundRPC)
func Test_raft_paper_election(t *testing.T) {
	tests := []struct {
		clusterSize  int
		voterToVoted map[uint64]bool

		wState raftpb.NODE_STATE
	}{
		{1, map[uint64]bool{}, raftpb.NODE_STATE_LEADER},                                   // 1 node cluster, so the one node becomes leader
		{3, map[uint64]bool{2: true, 3: true}, raftpb.NODE_STATE_LEADER},                   // voted by all
		{3, map[uint64]bool{2: true}, raftpb.NODE_STATE_LEADER},                            // voted by quorum
		{5, map[uint64]bool{2: true, 3: true, 4: true, 5: true}, raftpb.NODE_STATE_LEADER}, // voted by all
		{5, map[uint64]bool{2: true, 3: true, 4: true}, raftpb.NODE_STATE_LEADER},          // voted by quorum
		{5, map[uint64]bool{2: true, 3: true}, raftpb.NODE_STATE_LEADER},                   // voted by quorum

		{3, map[uint64]bool{2: false, 3: false}, raftpb.NODE_STATE_FOLLOWER}, // return to follower
		{5, map[uint64]bool{2: false, 3: false, 4: false, 5: false}, raftpb.NODE_STATE_FOLLOWER},
		{5, map[uint64]bool{2: true, 3: false, 4: false, 5: false}, raftpb.NODE_STATE_FOLLOWER},

		// stay in candidate
		{3, map[uint64]bool{}, raftpb.NODE_STATE_CANDIDATE},
		{5, map[uint64]bool{}, raftpb.NODE_STATE_CANDIDATE},
		{5, map[uint64]bool{2: true}, raftpb.NODE_STATE_CANDIDATE},
		{5, map[uint64]bool{2: true, 3: false}, raftpb.NODE_STATE_CANDIDATE},
	}

	for i, tt := range tests {
		rnd := newTestRaftNode(1, generateIDs(tt.clusterSize), 10, 1, NewStorageStableInMemory())
		oldTerm := rnd.currentTerm

		// trigger election in node 1
		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})

		for voterID, voted := range tt.voterToVoted {
			rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE, From: voterID, To: 1, Reject: !voted})
		}

		if rnd.state != tt.wState {
			t.Fatalf("#%d: node state expected %q, got %q", i, tt.wState, rnd.state)
		}

		if rnd.currentTerm != oldTerm+1 {
			t.Fatalf("#%d: term should have increased to %d, got %d", i, oldTerm+1, rnd.currentTerm)
		}
	}
}

// (etcd raft.TestFollowerVote)
func Test_raft_paper_follower_votes(t *testing.T) {
	tests := []struct {
		votedFor uint64
		voterTo1 uint64

		wReject bool
	}{
		{NoNodeID, 1, false},
		{NoNodeID, 2, false},
		{1, 1, false},
		{2, 2, false},
		{1, 2, true},
		{2, 1, true},
	}

	for i, tt := range tests {
		rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
		rnd.loadHardState(raftpb.HardState{Term: 1, VotedFor: tt.votedFor})

		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE, From: tt.voterTo1, To: 1, SenderCurrentTerm: 1})

		msgs := rnd.readAndClearMailbox()
		wmsgs := []raftpb.Message{
			{Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE, From: 1, To: tt.voterTo1, SenderCurrentTerm: 1, Reject: tt.wReject},
		}

		if !reflect.DeepEqual(msgs, wmsgs) {
			t.Fatalf("#%d: messages expected %+v, got %+v", i, wmsgs, msgs)
		}
	}
}

// (etcd raft.TestCandidateFallback)
func Test_raft_paper_candidate_revert_to_follower(t *testing.T) {
	tests := []raftpb.Message{
		{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, From: 2, To: 1, SenderCurrentTerm: 1}, // same term
		{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, From: 2, To: 1, SenderCurrentTerm: 2}, // bigger term
	}

	for i, msg := range tests {
		rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

		// trigger election in node 1
		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})

		rnd.assertNodeState(raftpb.NODE_STATE_CANDIDATE)

		if rnd.currentTerm != 1 {
			t.Fatalf("#%d: term expected 1, got %d", i, rnd.currentTerm)
		}

		rnd.Step(msg)

		rnd.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

		if rnd.currentTerm != msg.SenderCurrentTerm {
			t.Fatalf("#%d: term expected %d, got %d", i, msg.SenderCurrentTerm, rnd.currentTerm)
		}
	}
}

// (etcd raft.TestFollowerElectionTimeoutRandomized)
func Test_raft_paper_follower_election_timeout_randomized(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	electionTimedoutTick := make(map[int]bool)
	for i := 0; i < 50*rnd.electionTimeoutTickNum; i++ {
		rnd.becomeFollower(rnd.currentTerm+1, 2) // leader is 2

		time := 0
		for (len(rnd.readAndClearMailbox())) == 0 { // till election timeout
			rnd.tickFunc()
			time++
		}
		electionTimedoutTick[time] = true
	}

	for i := rnd.electionTimeoutTickNum + 1; i < 2*rnd.electionTimeoutTickNum; i++ {
		if !electionTimedoutTick[i] {
			t.Fatalf("tick %d does not exist", i)
		}
	}
}

// (etcd raft.TestCandidateElectionTimeoutRandomized)
func Test_raft_paper_candidate_election_timeout_randomized(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	electionTimedoutTick := make(map[int]bool)
	for i := 0; i < 50*rnd.electionTimeoutTickNum; i++ {
		rnd.becomeCandidate()

		time := 0
		for (len(rnd.readAndClearMailbox())) == 0 { // till election timeout
			rnd.tickFunc()
			time++
		}
		electionTimedoutTick[time] = true
	}

	for i := rnd.electionTimeoutTickNum + 1; i < 2*rnd.electionTimeoutTickNum; i++ {
		if !electionTimedoutTick[i] {
			t.Fatalf("tick %d does not exist", i)
		}
	}
}

// (etcd raft.TestFollowersElectioinTimeoutNonconflict)
func Test_raft_paper_follower_election_timeout_no_conflict(t *testing.T) {
	raftLogger.SetLogger(xlog.NewLogger("raft", xlog.CRITICAL))
	defer func() {
		raftLogger.SetLogger(xlog.NewLogger("raft", defaultLogLevel))
	}()

	raftNodes := make([]*raftNode, 5)
	ids := generateIDs(5)
	for i := range raftNodes {
		raftNodes[i] = newTestRaftNode(ids[i], ids, 10, 1, NewStorageStableInMemory())
	}

	timeoutConflict := 0

	for i := 0; i < 1000; i++ {
		for _, rnd := range raftNodes {
			rnd.becomeFollower(rnd.currentTerm+1, NoNodeID)
		}

		electionTimedout := 0
		for electionTimedout == 0 {
			for _, rnd := range raftNodes {
				rnd.tickFunc()
				if len(rnd.readAndClearMailbox()) > 0 { // election timed out
					electionTimedout++
				}
			}
		}

		if electionTimedout > 1 {
			timeoutConflict++
		}
	}

	prob := float64(timeoutConflict) / 1000
	if prob > 0.3 {
		t.Fatalf("too much timeout conflicts, got %f (conflicts %d)", prob, timeoutConflict)
	}
}

// (etcd raft.TestCandidatesElectionTimeoutNonconflict)
func Test_raft_paper_candidate_election_timeout_no_conflict(t *testing.T) {
	raftLogger.SetLogger(xlog.NewLogger("raft", xlog.CRITICAL))
	defer func() {
		raftLogger.SetLogger(xlog.NewLogger("raft", defaultLogLevel))
	}()

	raftNodes := make([]*raftNode, 5)
	ids := generateIDs(5)
	for i := range raftNodes {
		raftNodes[i] = newTestRaftNode(ids[i], ids, 10, 1, NewStorageStableInMemory())
	}

	timeoutConflict := 0

	for i := 0; i < 1000; i++ {
		for _, rnd := range raftNodes {
			rnd.becomeCandidate()
		}

		electionTimedout := 0
		for electionTimedout == 0 {
			for _, rnd := range raftNodes {
				rnd.tickFunc()
				if len(rnd.readAndClearMailbox()) > 0 { // election timed out
					electionTimedout++
				}
			}
		}

		if electionTimedout > 1 {
			timeoutConflict++
		}
	}

	prob := float64(timeoutConflict) / 1000
	if prob > 0.3 {
		t.Fatalf("too much timeout conflicts, got %f (conflicts %d)", prob, timeoutConflict)
	}
}

// (etcd raft.TestLeaderStartReplication)
func Test_raft_paper_leader_start_replication(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd.becomeCandidate()
	rnd.becomeLeader()

	rnd.commitAll()
	lastIndex1 := rnd.storageRaftLog.lastIndex()

	rnd.Step(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		From:    1,
		To:      1,
		Entries: []raftpb.Entry{{Data: []byte("testdata")}},
	})

	lastIndex2 := rnd.storageRaftLog.lastIndex()

	if lastIndex2 != lastIndex1+1 {
		t.Fatalf("last index expected %d, got %d", lastIndex1+1, lastIndex2)
	}
	if rnd.storageRaftLog.committedIndex != lastIndex1 { // after commit
		t.Fatalf("committed index expected %d, got %d", lastIndex1, rnd.storageRaftLog.committedIndex)
	}

	msgs := rnd.readAndClearMailbox()
	sort.Sort(messageSlice(msgs))

	wmsgs := []raftpb.Message{
		{
			Type:                        raftpb.MESSAGE_TYPE_LEADER_APPEND,
			From:                        1,
			To:                          2,
			LogIndex:                    lastIndex1,
			LogTerm:                     1,
			SenderCurrentCommittedIndex: lastIndex1,
			SenderCurrentTerm:           1,
			Entries:                     []raftpb.Entry{{Index: lastIndex2, Term: 1, Data: []byte("testdata")}},
		},
		{
			Type:                        raftpb.MESSAGE_TYPE_LEADER_APPEND,
			From:                        1,
			To:                          3,
			LogIndex:                    lastIndex1,
			LogTerm:                     1,
			SenderCurrentCommittedIndex: lastIndex1,
			SenderCurrentTerm:           1,
			Entries:                     []raftpb.Entry{{Index: lastIndex2, Term: 1, Data: []byte("testdata")}},
		},
	}

	if !reflect.DeepEqual(msgs, wmsgs) {
		t.Fatalf("messages expected %+v, got %+v", wmsgs, msgs)
	}
}

// (etcd raft.TestLeaderCommitEntry)
func Test_raft_paper_leader_commit_entry(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd.becomeCandidate()
	rnd.becomeLeader()

	rnd.commitAll()
	lastIndex1 := rnd.storageRaftLog.lastIndex()

	rnd.Step(raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		From:    1,
		To:      1,
		Entries: []raftpb.Entry{{Data: []byte("testdata")}},
	})

	msgs := rnd.readAndClearMailbox()
	for _, msg := range msgs { // MsgApp
		rnd.Step(createAppendResponseMessage(msg))
	}
	// MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND increments index

	lastIndex2 := rnd.storageRaftLog.lastIndex()

	if lastIndex2 != lastIndex1+1 {
		t.Fatalf("last index expected %d, got %d", lastIndex1+1, lastIndex2)
	}
	if rnd.storageRaftLog.committedIndex != lastIndex2 { // after commit
		t.Fatalf("committed index expected %d, got %d", lastIndex2, rnd.storageRaftLog.committedIndex)
	}

	nextEntsToApply := rnd.storageRaftLog.nextEntriesToApply()
	wents := []raftpb.Entry{{Index: lastIndex2, Term: 1, Data: []byte("testdata")}}
	if !reflect.DeepEqual(nextEntsToApply, wents) {
		t.Fatalf("entries to apply expected %+v, got %+v", wents, nextEntsToApply)
	}

	msgs = rnd.readAndClearMailbox()
	sort.Sort(messageSlice(msgs))

	wmsgs := []raftpb.Message{
		{
			Type:                        raftpb.MESSAGE_TYPE_LEADER_APPEND,
			From:                        1,
			To:                          2,
			LogIndex:                    lastIndex2,
			LogTerm:                     1,
			SenderCurrentCommittedIndex: lastIndex2,
			SenderCurrentTerm:           1,
		},
		{
			Type:                        raftpb.MESSAGE_TYPE_LEADER_APPEND,
			From:                        1,
			To:                          3,
			LogIndex:                    lastIndex2,
			LogTerm:                     1,
			SenderCurrentCommittedIndex: lastIndex2,
			SenderCurrentTerm:           1,
		},
	}

	if !reflect.DeepEqual(msgs, wmsgs) {
		t.Fatalf("messages expected %+v, got %+v", wmsgs, msgs)
	}
}

// (etcd raft.TestLeaderAcknowledgeCommit)
func Test_raft_paper_leader_acknowledge_commit(t *testing.T) {
	tests := []struct {
		clusterSize  int
		idsToRespond map[uint64]bool

		wAck bool
	}{
		{1, nil, true},

		{3, nil, false},
		{3, map[uint64]bool{2: true}, true},
		{3, map[uint64]bool{2: true, 3: true}, true},

		{5, nil, false},
		{5, map[uint64]bool{2: true}, false},
		{5, map[uint64]bool{2: true, 3: true}, true},
		{5, map[uint64]bool{2: true, 3: true, 4: true}, true},
		{5, map[uint64]bool{2: true, 3: true, 4: true, 5: true}, true},
	}

	for i, tt := range tests {
		rnd := newTestRaftNode(1, generateIDs(tt.clusterSize), 10, 1, NewStorageStableInMemory())
		rnd.becomeCandidate()
		rnd.becomeLeader()

		rnd.commitAll()
		lastIndex1 := rnd.storageRaftLog.lastIndex()

		rnd.Step(raftpb.Message{
			Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
			From:    1,
			To:      1,
			Entries: []raftpb.Entry{{Data: []byte("testdata")}},
		})

		lastIndex2 := rnd.storageRaftLog.lastIndex()

		if lastIndex2 != lastIndex1+1 {
			t.Fatalf("#%d: last index expected %d, got %d", i, lastIndex1+1, lastIndex2)
		}

		for _, msg := range rnd.readAndClearMailbox() {
			if tt.idsToRespond[msg.To] {
				rnd.Step(createAppendResponseMessage(msg))
			}
		}

		ack := rnd.storageRaftLog.committedIndex > lastIndex1
		if ack != tt.wAck {
			t.Fatalf("#%d: ack expected %v, got %v", i, tt.wAck, ack)
		}
	}
}

// (etcd raft.TestLeaderCommitPrecedingEntries)
func Test_raft_paper_leader_commit_preceding_entries(t *testing.T) {
	tests := [][]raftpb.Entry{
		{},
		{{Index: 1, Term: 2}},
		{{Index: 1, Term: 1}, {Index: 2, Term: 2}},
		{{Index: 1, Term: 1}},
	}

	for i, tt := range tests {
		st := NewStorageStableInMemory()
		st.Append(tt...)

		rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, st)
		rnd.loadHardState(raftpb.HardState{Term: 2})

		rnd.becomeCandidate()
		rnd.becomeLeader()

		rnd.Step(raftpb.Message{
			Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
			From:    1,
			To:      1,
			Entries: []raftpb.Entry{{Data: []byte("testdata")}},
		})

		for _, msg := range rnd.readAndClearMailbox() {
			rnd.Step(createAppendResponseMessage(msg))
		}

		nextEntsToApply := rnd.storageRaftLog.nextEntriesToApply()

		entriesN := uint64(len(tt))

		wents := append(tt, raftpb.Entry{Index: entriesN + 1, Term: 3}, raftpb.Entry{Index: entriesN + 2, Term: 3, Data: []byte("testdata")})

		if !reflect.DeepEqual(nextEntsToApply, wents) {
			t.Fatalf("#%d: entries to apply expected %+v, got %+v", i, wents, nextEntsToApply)
		}
	}
}

// (etcd raft.TestFollowerCommitEntry)
func Test_raft_paper_follower_commit_entries(t *testing.T) {
	tests := []struct {
		ents           []raftpb.Entry
		committedIndex uint64
	}{
		{
			[]raftpb.Entry{
				{Index: 1, Term: 1, Data: []byte("testdata")},
			},
			1,
		},
		{
			[]raftpb.Entry{
				{Index: 1, Term: 1, Data: []byte("testdata")},
				{Index: 2, Term: 1, Data: []byte("testdata2")},
			},
			2,
		},
		{
			[]raftpb.Entry{
				{Index: 1, Term: 1, Data: []byte("testdata2")},
				{Index: 2, Term: 1, Data: []byte("testdata")},
			},
			2,
		},
		{
			[]raftpb.Entry{
				{Index: 1, Term: 1, Data: []byte("testdata")},
				{Index: 2, Term: 1, Data: []byte("testdata2")},
			},
			1,
		},
	}

	for i, tt := range tests {
		rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
		rnd.becomeFollower(1, 2)

		rnd.Step(raftpb.Message{
			From: 2,
			To:   1,
			Type: raftpb.MESSAGE_TYPE_LEADER_APPEND,
			SenderCurrentCommittedIndex: tt.committedIndex,
			SenderCurrentTerm:           1,
			Entries:                     tt.ents,
		})

		if rnd.storageRaftLog.committedIndex != tt.committedIndex {
			t.Fatalf("#%d: committed index expected %d, got %d", i, tt.committedIndex, rnd.storageRaftLog.committedIndex)
		}

		nextEntsToApply := rnd.storageRaftLog.nextEntriesToApply()
		wents := tt.ents[:int(tt.committedIndex)]

		if !reflect.DeepEqual(nextEntsToApply, wents) {
			t.Fatalf("#%d: entries to apply expected %+v, got %+v", i, wents, nextEntsToApply)
		}
	}
}

// (etcd raft.TestFollowerCheckMsgApp)
func Test_raft_paper_follower_check_leader_append(t *testing.T) {
	ents := []raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}}

	tests := []struct {
		term  uint64
		index uint64

		wLogIndex   uint64
		wReject     bool
		wRejectHint uint64
	}{

		{0, 0, 1, false, 0}, // match with committed entries
		{ents[0].Term, ents[0].Index, 1, false, 0},
		{ents[1].Term, ents[1].Index, 2, false, 0}, // match with uncommitted entries

		{ents[0].Term, ents[1].Index, ents[1].Index, true, 2},             // unmatch with existing entry
		{ents[1].Term + 1, ents[1].Index + 1, ents[1].Index + 1, true, 2}, // unexisting entry
	}

	for i, tt := range tests {
		st := NewStorageStableInMemory()
		st.Append(ents...)

		rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, st)
		rnd.loadHardState(raftpb.HardState{CommittedIndex: 1})
		rnd.becomeFollower(2, 2)

		rnd.Step(raftpb.Message{
			From:              2,
			To:                1,
			Type:              raftpb.MESSAGE_TYPE_LEADER_APPEND,
			SenderCurrentTerm: 2,
			LogIndex:          tt.index,
			LogTerm:           tt.term,
		})

		msgs := rnd.readAndClearMailbox()
		wmsgs := []raftpb.Message{
			{
				From:              1,
				To:                2,
				Type:              raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND,
				SenderCurrentTerm: 2,
				LogIndex:          tt.wLogIndex,
				Reject:            tt.wReject,
				RejectHintFollowerLogLastIndex: tt.wRejectHint,
			},
		}
		if !reflect.DeepEqual(msgs, wmsgs) {
			t.Fatalf("#%d: messages expected %+v, got %+v", i, wmsgs, msgs)
		}
	}
}

// (etcd raft.TestFollowerAppendEntries)
func Test_raft_paper_follower_append_entries(t *testing.T) {
	tests := []struct {
		logIndex, logTerm uint64
		ents              []raftpb.Entry

		wAllEnts         []raftpb.Entry
		wUnstableEntries []raftpb.Entry
	}{
		{
			2, 2,
			[]raftpb.Entry{{Index: 3, Term: 3}},

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}},
			[]raftpb.Entry{{Index: 3, Term: 3}},
		},
		{
			1, 1,
			[]raftpb.Entry{{Index: 2, Term: 3}, {Index: 3, Term: 4}},

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 3}, {Index: 3, Term: 4}},
			[]raftpb.Entry{{Index: 2, Term: 3}, {Index: 3, Term: 4}},
		},
		{
			0, 0,
			[]raftpb.Entry{{Index: 1, Term: 1}},

			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},
			nil,
		},
		{
			0, 0,
			[]raftpb.Entry{{Index: 1, Term: 3}},

			[]raftpb.Entry{{Index: 1, Term: 3}},
			[]raftpb.Entry{{Index: 1, Term: 3}},
		},
	}
	for i, tt := range tests {
		st := NewStorageStableInMemory()
		st.Append([]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}}...)

		rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, st)
		rnd.becomeFollower(2, 2)

		rnd.Step(raftpb.Message{
			From:              2,
			To:                1,
			Type:              raftpb.MESSAGE_TYPE_LEADER_APPEND,
			SenderCurrentTerm: 2,
			LogIndex:          tt.logIndex,
			LogTerm:           tt.logTerm,
			Entries:           tt.ents,
		})

		allEnts := rnd.storageRaftLog.allEntries()
		if !reflect.DeepEqual(allEnts, tt.wAllEnts) {
			t.Fatalf("#%d: all entries expected %+v, got %+v", i, tt.wAllEnts, allEnts)
		}

		unstableEnts := rnd.storageRaftLog.unstableEntries()
		if !reflect.DeepEqual(unstableEnts, tt.wUnstableEntries) {
			t.Fatalf("#%d: unstable entries expected %+v, got %+v", i, tt.wUnstableEntries, unstableEnts)
		}
	}
}

// (etcd raft.TestLeaderSyncFollowerLog)
func Test_raft_paper_leader_sync_follower_log(t *testing.T) {
	ents := []raftpb.Entry{
		{},
		{Index: 1, Term: 1}, {Index: 2, Term: 1}, {Index: 3, Term: 1},
		{Index: 4, Term: 4}, {Index: 5, Term: 4},
		{Index: 6, Term: 5}, {Index: 7, Term: 5},
		{Index: 8, Term: 6}, {Index: 9, Term: 6}, {Index: 10, Term: 6},
	}

	term := uint64(8)

	tests := [][]raftpb.Entry{
		{
			{},
			{Index: 1, Term: 1}, {Index: 2, Term: 1}, {Index: 3, Term: 1},
			{Index: 4, Term: 4}, {Index: 5, Term: 4},
			{Index: 6, Term: 5}, {Index: 7, Term: 5},
			{Index: 8, Term: 6}, {Index: 9, Term: 6},
		},
		{
			{},
			{Index: 1, Term: 1}, {Index: 2, Term: 1}, {Index: 3, Term: 1},
			{Index: 4, Term: 4},
		},
		{
			{},
			{Index: 1, Term: 1}, {Index: 2, Term: 1}, {Index: 3, Term: 1},
			{Index: 4, Term: 4}, {Index: 5, Term: 4},
			{Index: 6, Term: 5}, {Index: 7, Term: 5},
			{Index: 8, Term: 6}, {Index: 9, Term: 6}, {Index: 10, Term: 6}, {Index: 11, Term: 6},
		},
		{
			{},
			{Index: 1, Term: 1}, {Index: 2, Term: 1}, {Index: 3, Term: 1},
			{Index: 4, Term: 4}, {Index: 5, Term: 4},
			{Index: 6, Term: 5}, {Index: 7, Term: 5},
			{Index: 8, Term: 6}, {Index: 9, Term: 6}, {Index: 10, Term: 6},
			{Index: 11, Term: 7}, {Index: 12, Term: 7},
		},
		{
			{},
			{Index: 1, Term: 1}, {Index: 2, Term: 1}, {Index: 3, Term: 1},
			{Index: 4, Term: 4}, {Index: 5, Term: 4}, {Index: 6, Term: 4}, {Index: 7, Term: 4},
		},
		{
			{},
			{Index: 1, Term: 1}, {Index: 2, Term: 1}, {Index: 3, Term: 1},
			{Index: 4, Term: 2}, {Index: 5, Term: 2}, {Index: 6, Term: 2},
			{Index: 7, Term: 3}, {Index: 8, Term: 3}, {Index: 9, Term: 3}, {Index: 10, Term: 3}, {Index: 11, Term: 3},
		},
	}
	for i, tt := range tests {
		stLeader := NewStorageStableInMemory()
		stLeader.Append(ents...)
		rndLeader := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, stLeader)
		rndLeader.loadHardState(raftpb.HardState{CommittedIndex: rndLeader.storageRaftLog.lastIndex(), Term: term})

		stFollower := NewStorageStableInMemory()
		stFollower.Append(tt...)
		rndFollower := newTestRaftNode(2, []uint64{1, 2, 3}, 10, 1, stFollower)
		rndFollower.loadHardState(raftpb.HardState{Term: term - 1})

		fn := newFakeNetwork(rndLeader, rndFollower, noOpBlackHole)

		fn.stepFirstMessage(raftpb.Message{
			Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
			From: 1,
			To:   1,
		})
		fn.stepFirstMessage(raftpb.Message{
			Type:              raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE,
			From:              3,
			To:                1,
			SenderCurrentTerm: 1,
		})
		fn.stepFirstMessage(raftpb.Message{
			Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
			From:    1,
			To:      1,
			Entries: []raftpb.Entry{{}},
		})

		if g := diffu(ltoa(rndLeader.storageRaftLog), ltoa(rndFollower.storageRaftLog)); g != "" {
			t.Fatalf("#%d: log diff:\n%s", i, g)
		}
	}
}

// (etcd raft.TestVoteRequest)
func Test_raft_paper_vote_request(t *testing.T) {
	tests := []struct {
		ents  []raftpb.Entry
		wTerm uint64
	}{
		{
			[]raftpb.Entry{{Index: 1, Term: 1}},
			2,
		},
		{
			[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},
			3,
		},
	}

	for i, tt := range tests {
		rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
		rnd.Step(raftpb.Message{
			Type:              raftpb.MESSAGE_TYPE_LEADER_APPEND,
			From:              2,
			To:                1,
			SenderCurrentTerm: tt.wTerm - 1,
			LogIndex:          0,
			LogTerm:           0,
			Entries:           tt.ents,
		})
		rnd.readAndClearMailbox()

		for j := 0; j < 2*rnd.electionTimeoutTickNum; j++ {
			rnd.tickFunc()
		}

		msgs := rnd.readAndClearMailbox()
		sort.Sort(messageSlice(msgs))
		if len(msgs) != 2 {
			t.Fatalf("#%d: len(msg) expected %d, want %d", i, 2, len(msgs))
		}

		for j, msg := range msgs {
			if msg.Type != raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE {
				t.Fatalf("#%d: msg type expected %q, got %q", j, raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE, msg.Type)
			}
			if msg.To != uint64(j+2) {
				t.Fatalf("#%d: to expected %d, got %d", j, j+2, msg.To)
			}
			if msg.SenderCurrentTerm != tt.wTerm {
				t.Fatalf("#%d: SenderCurrentTerm expected %d, got %d", j, tt.wTerm, msg.SenderCurrentTerm)
			}

			wLogIndex, wLogTerm := tt.ents[len(tt.ents)-1].Index, tt.ents[len(tt.ents)-1].Term
			if msg.LogIndex != wLogIndex {
				t.Fatalf("#%d: index expected %d, got %d", j, wLogIndex, msg.LogIndex)
			}
			if msg.LogTerm != wLogTerm {
				t.Fatalf("#%d: logterm expected %d, got %d", j, wLogTerm, msg.LogTerm)
			}
		}
	}
}

// (etcd raft.TestVoter)
func Test_raft_paper_voter(t *testing.T) {
	tests := []struct {
		ents     []raftpb.Entry
		logTerm  uint64
		logIndex uint64

		wReject bool
	}{
		// same logterm
		{[]raftpb.Entry{{Index: 1, Term: 1}}, 1, 1, false},
		{[]raftpb.Entry{{Index: 1, Term: 1}}, 1, 2, false},
		{[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 1}}, 1, 1, true},

		// candidate higher logterm
		{[]raftpb.Entry{{Index: 1, Term: 1}}, 2, 1, false},
		{[]raftpb.Entry{{Index: 1, Term: 1}}, 2, 2, false},
		{[]raftpb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 1}}, 2, 1, false},

		// voter higher logterm
		{[]raftpb.Entry{{Index: 1, Term: 2}}, 1, 1, true},
		{[]raftpb.Entry{{Index: 1, Term: 2}}, 1, 2, true},
		{[]raftpb.Entry{{Index: 1, Term: 2}, {Index: 2, Term: 1}}, 1, 1, true},
	}

	for i, tt := range tests {
		st := NewStorageStableInMemory()
		st.Append(tt.ents...)
		rnd := newTestRaftNode(1, []uint64{1, 2}, 10, 1, st)

		rnd.Step(raftpb.Message{
			Type:              raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE,
			From:              2,
			To:                1,
			SenderCurrentTerm: 3,
			LogIndex:          tt.logIndex,
			LogTerm:           tt.logTerm,
		})

		msgs := rnd.readAndClearMailbox()
		if len(msgs) != 1 {
			t.Fatalf("#%d: len(msg) expected 1, got %d", i, len(msgs))
		}

		msg := msgs[0]
		if msg.Type != raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE {
			t.Fatalf("#%d: msg type expected %q, got %q", i, raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE, msg.Type)
		}

		if msg.Reject != tt.wReject {
			t.Fatalf("#%d: reject expected %v, got %v", i, tt.wReject, msg.Reject)
		}
	}
}

// (etcd raft.TestLeaderOnlyCommitsLogFromCurrentTerm)
func Test_raft_paper_leader_commits_from_current_term(t *testing.T) {

}
