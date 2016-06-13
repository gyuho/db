package raftpb

var (
	// EmptyHardState is an empty hard state.
	EmptyHardState = HardState{}
)

// checkHardStateEqual returns true if two states are equal
func checkHardStateEqual(a, b HardState) bool {
	return a.Term == b.Term && a.VotedFor == b.VotedFor && a.CommitIndex == b.CommitIndex
}

// IsEmptyHardState returns true if the given HardState is empty.
func IsEmptyHardState(st HardState) bool {
	return checkHardStateEqual(st, EmptyHardState)
}

// MustStoreHardState returns true, if the given hard state must be
// stored in the disk. Raft stores the state on all servers before
// responding to RPCs.
func MustStoreHardState(prev, cur HardState, entN int) bool {
	// different, so we need to store the HardState to disk
	return entN != 0 || prev.Term != cur.Term || prev.VotedFor != cur.VotedFor
}
