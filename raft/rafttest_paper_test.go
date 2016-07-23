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

	// case msg.SenderCurrentTerm > rnd.term:
	//
	// func (rnd *raftNode) sendToMailbox(msg raftpb.Message) {
	//   msg.From = rnd.id
	//
	rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, SenderCurrentTerm: 2})

	if rnd.term != 2 {
		t.Fatalf("term expected 2, got %d", rnd.term)
	}
	if rnd.state != raftpb.NODE_STATE_FOLLOWER {
		t.Fatalf("node state expected %q, got %q", raftpb.NODE_STATE_FOLLOWER, rnd.state)
	}
}

// (etcd raft.TestCandidateUpdateTermFromMessage)
func Test_raft_paper_candidate_update_term_from_Message(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())
	rnd.becomeCandidate()

	// case msg.SenderCurrentTerm > rnd.term:
	//
	// func (rnd *raftNode) sendToMailbox(msg raftpb.Message) {
	//   msg.From = rnd.id
	//
	rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, SenderCurrentTerm: 2})

	if rnd.term != 2 {
		t.Fatalf("term expected 2, got %d", rnd.term)
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

	// case msg.SenderCurrentTerm > rnd.term:
	//
	// func (rnd *raftNode) sendToMailbox(msg raftpb.Message) {
	//   msg.From = rnd.id
	//
	rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, SenderCurrentTerm: 2})

	if rnd.term != 2 {
		t.Fatalf("term expected 2, got %d", rnd.term)
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

	rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, SenderCurrentTerm: rnd.term - 1})

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
	oldTerm := rnd.term

	// election timeout to trigger election from follower
	for i := 0; i < 2*rnd.electionTimeoutTickNum; i++ {
		rnd.tickFunc()
	}

	if rnd.term != oldTerm+1 {
		t.Fatalf("term should have increased to %d, got %d", oldTerm+1, rnd.term)
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
	oldTerm := rnd.term

	// election timeout to trigger another election from candidate
	// candidate starts a new election and increments its term
	for i := 0; i < 2*rnd.electionTimeoutTickNum; i++ {
		rnd.tickFunc()
	}

	if rnd.term != oldTerm+1 {
		t.Fatalf("term should have increased to %d, got %d", oldTerm+1, rnd.term)
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
		oldTerm := rnd.term

		// trigger election in node 1
		rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, From: 1, To: 1})

		for voterID, voted := range tt.voterToVoted {
			rnd.Step(raftpb.Message{Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE, From: voterID, To: 1, Reject: !voted})
		}

		if rnd.state != tt.wState {
			t.Fatalf("#%d: node state expected %q, got %q", i, tt.wState, rnd.state)
		}

		if rnd.term != oldTerm+1 {
			t.Fatalf("#%d: term should have increased to %d, got %d", i, oldTerm+1, rnd.term)
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

		if rnd.term != 1 {
			t.Fatalf("#%d: term expected 1, got %d", i, rnd.term)
		}

		rnd.Step(msg)

		rnd.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

		if rnd.term != msg.SenderCurrentTerm {
			t.Fatalf("#%d: term expected %d, got %d", i, msg.SenderCurrentTerm, rnd.term)
		}
	}
}

// (etcd raft.TestFollowerElectionTimeoutRandomized)
func Test_raft_paper_follower_election_timeout_randomized(t *testing.T) {
	rnd := newTestRaftNode(1, []uint64{1, 2, 3}, 10, 1, NewStorageStableInMemory())

	electionTimedoutTick := make(map[int]bool)
	for i := 0; i < 50*rnd.electionTimeoutTickNum; i++ {
		rnd.becomeFollower(rnd.term+1, 2) // leader is 2

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
		raftLogger.SetLogger(xlog.NewLogger("raft", xlog.INFO))
	}()

	raftNodes := make([]*raftNode, 5)
	ids := generateIDs(5)
	for i := range raftNodes {
		raftNodes[i] = newTestRaftNode(ids[i], ids, 10, 1, NewStorageStableInMemory())
	}

	timeoutConflict := 0

	for i := 0; i < 1000; i++ {
		for _, rnd := range raftNodes {
			rnd.becomeFollower(rnd.term+1, NoNodeID)
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
		raftLogger.SetLogger(xlog.NewLogger("raft", xlog.INFO))
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

// (etcd raft.TestLeaderCommitPrecedingEntries)

// (etcd raft.TestFollowerCommitEntry)

// (etcd raft.TestFollowerCheckMsgApp)

// (etcd raft.TestFollowerAppendEntries)

// (etcd raft.TestLeaderSyncFollowerLog)

// (etcd raft.TestVoteRequest)

// (etcd raft.TestVoter)

// (etcd raft.TestLeaderOnlyCommitsLogFromCurrentTerm)
