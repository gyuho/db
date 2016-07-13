package raft

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

func Test_Step_campaign(t *testing.T) {
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
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_FOLLOWER_OR_CANDIDATE_TO_START_CAMPAIGN,
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
