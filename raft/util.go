package raft

type uint64Slice []uint64

func (s uint64Slice) Len() int           { return len(s) }
func (s uint64Slice) Less(i, j int) bool { return s[i] < s[j] }
func (s uint64Slice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func minUint64(a, b uint64) uint64 {
	if a > b {
		return b
	}
	return a
}

func maxUint64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
