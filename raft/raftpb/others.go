package raftpb

import (
	"bytes"
	"fmt"
)

// IsEmptySnapshot returns true if the given Snapshot is empty.
func IsEmptySnapshot(snap Snapshot) bool {
	return snap.Metadata.Index == 0
}

// Equal returns true, if two SoftState-s are equal.
func (s *SoftState) Equal(st *SoftState) bool {
	return s.LeaderID == st.LeaderID && s.NodeState == st.NodeState
}

// checkHardState returns true if two states are equal
func checkHardState(a, b HardState) bool {
	return a.CommittedIndex == b.CommittedIndex && a.Term == b.Term && a.VotedFor == b.VotedFor
}

// EmptyHardState is an empty hard state.
var EmptyHardState = HardState{}

// IsEmptyHardState returns true if the given HardState is empty.
func IsEmptyHardState(st HardState) bool {
	return checkHardState(st, EmptyHardState)
}

// IsResponseMessage returns true if the message type is response.
//
// (etcd raft.IsResponseMsg)
func IsResponseMessage(tp MESSAGE_TYPE) bool {
	return tp == MESSAGE_TYPE_APPEND_RESPONSE ||
		tp == MESSAGE_TYPE_VOTE_RESPONSE ||
		tp == MESSAGE_TYPE_HEARTBEAT_RESPONSE ||
		tp == MESSAGE_TYPE_INTERNAL_UNREACHABLE_FOLLOWER
}

// IsInternalMessage returns true if the message type is internal.
//
// (etcd raft.IsLocalMsg)
func IsInternalMessage(tp MESSAGE_TYPE) bool {
	return tp == MESSAGE_TYPE_INTERNAL_CAMPAIGN_START ||
		tp == MESSAGE_TYPE_INTERNAL_LEADER_SEND_HEARTBEAT ||
		tp == MESSAGE_TYPE_INTERNAL_UNREACHABLE_FOLLOWER ||
		tp == MESSAGE_TYPE_INTERNAL_SNAPSHOT_RESPONSE ||
		tp == MESSAGE_TYPE_INTERNAL_CHECK_QUORUM ||
		tp == MESSAGE_TYPE_INTERNAL_LEADER_TRANSFER
}

// DescribeEntry describes Entry in human-readable format.
//
// (etcd raft.DescribeEntry)
func DescribeEntry(e Entry) string {
	return fmt.Sprintf("[index=%d | term=%d | type=%q | data=%q]", e.Index, e.Term, e.Type, e.Data)
}

// DescribeMessage describes Message in human-readable format.
//
// (etcd raft.DescribeMessage)
func DescribeMessage(msg Message) string {
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "Message [type=%q | from=%X ➝ to=%X | current committed index=%d, current term=%d | log index=%d, log term=%d | reject=%v, reject hint follower last index=%d]",
		msg.Type, msg.From, msg.To, msg.CurrentCommittedIndex, msg.CurrentTerm, msg.LogIndex, msg.LogTerm, msg.Reject, msg.RejectHintFollowerLastIndex)

	if len(msg.Entries) > 0 {
		buf.WriteString(", Entries: [")
		for i, e := range msg.Entries {
			if i != 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(DescribeEntry(e))
		}
		buf.WriteString("]")
	}

	if !IsEmptySnapshot(msg.Snapshot) {
		fmt.Fprintf(buf, ", Snapshot: %+v", msg.Snapshot)
	}

	return buf.String()
}
