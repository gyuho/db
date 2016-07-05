package raft

import "github.com/gyuho/db/raft/raftpb"

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

func limitEntries(limitSize uint64, entries ...raftpb.Entry) []raftpb.Entry {
	if len(entries) == 0 {
		return entries
	}

	var (
		total int
		i     int
	)
	for i = 0; i < len(entries); i++ {
		total += entries[i].Size()

		// to return at least one entry
		if i != 0 && uint64(total) > limitSize {
			break
		}
	}

	return entries[:i]
}

type uint64Slice []uint64

func (s uint64Slice) Len() int           { return len(s) }
func (s uint64Slice) Less(i, j int) bool { return s[i] < s[j] }
func (s uint64Slice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
