package rafthttp

import (
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd rafthttp.TestPeerPick)
func Test_peer_pick(t *testing.T) {
	tests := []struct {
		messageWorking bool
		m              raftpb.Message
		wpicked        string
	}{
		{
			true,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT},
			messageTypePipeline,
		},
		{
			true,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, SenderCurrentTerm: 1, LogTerm: 1},
			messageTypeMessage,
		},
		{
			true,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER},
			messageTypeMessage,
		},
		{
			true,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT},
			messageTypeMessage,
		},
		{
			true,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, SenderCurrentTerm: 1, LogTerm: 1},
			messageTypeMessage,
		},
		{
			false,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_APPEND, SenderCurrentTerm: 1, LogTerm: 1},
			messageTypePipeline,
		},
		{
			false,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER},
			messageTypePipeline,
		},
		{
			false,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT},
			messageTypePipeline,
		},
		{
			false,
			raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT},
			messageTypePipeline,
		},
	}
	for i, tt := range tests {
		peer := &peer{
			streamWriter: &streamWriter{working: tt.messageWorking},
			pipeline:     &pipeline{},
		}

		_, picked := peer.pick(tt.m)
		if picked != tt.wpicked {
			t.Fatalf("#%d: picked expected %v, got %v", i, tt.wpicked, picked)
		}
	}
}
