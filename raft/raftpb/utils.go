package raftpb

import (
	"bytes"
	"fmt"
)

// IsEmptySnapshot returns true if the given Snapshot is empty.
//
// (etcd raft.IsEmptySnap)
func IsEmptySnapshot(snap Snapshot) bool {
	return snap.Metadata.Index == 0
}

// Equal returns true, if two SoftState-s are equal.
func (s *SoftState) Equal(st *SoftState) bool {
	return s.LeaderID == st.LeaderID && s.NodeState == st.NodeState
}

// Equal returns true if two states are equal.
//
// (etcd raft.isHardStateEqual)
func (a HardState) Equal(b HardState) bool {
	return a.CommittedIndex == b.CommittedIndex && a.Term == b.Term && a.VotedFor == b.VotedFor
}

// EmptyHardState is an empty hard state.
var EmptyHardState = HardState{}

// IsEmptyHardState returns true if the given HardState is empty.
//
// (etcd raft.IsEmptyHardState)
func IsEmptyHardState(st HardState) bool {
	return st.Equal(EmptyHardState)
}

// IsResponseMessage returns true if the message type is response.
//
// (etcd raft.IsResponseMsg)
func IsResponseMessage(tp MESSAGE_TYPE) bool {
	return tp == MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND ||
		tp == MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE ||
		tp == MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT ||
		tp == MESSAGE_TYPE_INTERNAL_LEADER_CANNOT_CONNECT_TO_FOLLOWER
}

// IsInternalMessage returns true if the message type is internal.
//
// (etcd raft.IsLocalMsg)
func IsInternalMessage(tp MESSAGE_TYPE) bool {
	return tp == MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN ||
		tp == MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT ||
		tp == MESSAGE_TYPE_INTERNAL_LEADER_CANNOT_CONNECT_TO_FOLLOWER ||
		tp == MESSAGE_TYPE_INTERNAL_RESPONSE_TO_LEADER_SNAPSHOT ||
		tp == MESSAGE_TYPE_INTERNAL_TRIGGER_CHECK_QUORUM ||
		tp == MESSAGE_TYPE_INTERNAL_LEADER_TRANSFER

	// ???
	// MESSAGE_TYPE_INTERNAL_RESPONSE_TO_LEADER_SNAPSHOT
}

// (etcd wal.mustSync)
func HardStateContainUpdates(prevHardState, hardState HardState, entryNum int) bool {
	return entryNum != 0 || prevHardState.VotedFor != hardState.VotedFor || prevHardState.Term != hardState.Term
}

// DescribeMessage decribes message.
func DescribeMessage(msg Message) string {
	return fmt.Sprintf(`%q [from=%x | message sender current term=%d]`, msg.Type, msg.From, msg.SenderCurrentTerm)
}

// DescribeMessageLong decribes message.
func DescribeMessageLong(msg Message) string {
	return fmt.Sprintf(`%q
	[from=%x ➝ to=%x | sender current committed index=%d | sender current term=%d]
	[log index=%d | log term=%d | reject=%v | reject hint follower last index=%d]
	[snapshot metadata index=%d | snapshot metadata term=%d]`,
		msg.Type, msg.From, msg.To, msg.SenderCurrentCommittedIndex, msg.SenderCurrentTerm,
		msg.LogIndex, msg.LogTerm, msg.Reject, msg.RejectHintFollowerLogLastIndex,
		msg.Snapshot.Metadata.Index, msg.Snapshot.Metadata.Term,
	)
}

// DescribeMessageLongLong describes Message in human-readable format.
//
// (etcd raft.DescribeMessage)
func DescribeMessageLongLong(msg Message) string {
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, `Message [type=%q | from=%x ➝ to=%x | sender current committed index=%d, sender current term=%d | log index=%d, log term=%d | reject=%v, reject hint follower last index=%d]`,
		msg.Type, msg.From, msg.To, msg.SenderCurrentCommittedIndex, msg.SenderCurrentTerm, msg.LogIndex, msg.LogTerm, msg.Reject, msg.RejectHintFollowerLogLastIndex)

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

// DescribeEntry describes Entry in human-readable format.
//
// (etcd raft.DescribeEntry)
func DescribeEntry(e Entry) string {
	return fmt.Sprintf("[index=%d | term=%d | type=%q | data=%q]", e.Index, e.Term, e.Type, e.Data)
}
