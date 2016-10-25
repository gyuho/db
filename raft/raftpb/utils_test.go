package raftpb

import (
	"fmt"
	"testing"
)

// (etcd raft.TestIsLocalMsg)
func Test_IsInternalMessage(t *testing.T) {
	tests := []struct {
		msgt       MESSAGE_TYPE
		isInternal bool
	}{
		{MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN, true},
		{MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT, true},
		{MESSAGE_TYPE_INTERNAL_LEADER_CANNOT_CONNECT_TO_FOLLOWER, true},
		{MESSAGE_TYPE_INTERNAL_RESPONSE_TO_LEADER_SNAPSHOT, true},
		{MESSAGE_TYPE_INTERNAL_TRIGGER_CHECK_QUORUM, true},
		{MESSAGE_TYPE_TRANSFER_LEADER, false},
		{MESSAGE_TYPE_PROPOSAL_TO_LEADER, false},
		{MESSAGE_TYPE_LEADER_APPEND, false},
		{MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND, false},
		{MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE, false},
		{MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE, false},
		{MESSAGE_TYPE_LEADER_SNAPSHOT, false},
		{MESSAGE_TYPE_LEADER_HEARTBEAT, false},
		{MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT, false},
		{MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT, false},
		{MESSAGE_TYPE_TRIGGER_READ_INDEX, false},
		{MESSAGE_TYPE_READ_INDEX_DATA, false},
		{MESSAGE_TYPE_PRE_CANDIDATE_REQUEST_VOTE, false},
		{MESSAGE_TYPE_RESPONSE_TO_PRE_CANDIDATE_REQUEST_VOTE, false},
	}

	for i, tt := range tests {
		got := IsInternalMessage(tt.msgt)
		if got != tt.isInternal {
			t.Errorf("#%d: got %v, want %v", i, got, tt.isInternal)
		}
	}
}

func Test_DescribeEntry(t *testing.T) {
	entry := Entry{
		Type:  ENTRY_TYPE_NORMAL,
		Index: 2,
		Term:  1,
		Data:  []byte("hello"),
	}
	fmt.Println(DescribeEntry(entry))
}

func Test_DescribeMessageLongLong(t *testing.T) {
	entry := Entry{
		Type:  ENTRY_TYPE_NORMAL,
		Index: 2,
		Term:  1,
		Data:  []byte("hello"),
	}
	msg := Message{
		Type:     MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND,
		From:     7777,
		To:       9999,
		LogIndex: 10,
		LogTerm:  5,
		Entries:  []Entry{entry},
	}
	fmt.Println(DescribeMessageLongLong(msg))
}
