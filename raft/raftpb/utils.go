package raftpb

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

// MustStoreHardState returns true, if the given hard state needs to be stored in the disk.
// Raft stores the state on all servers before responding to RPCs.
func MustStoreHardState(prev, cur HardState, entN int) bool {
	// different, then need to store the HardState to disk
	return entN != 0 || prev.Term != cur.Term || prev.VotedFor != cur.VotedFor
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
