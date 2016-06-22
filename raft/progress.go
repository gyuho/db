package raft

import "github.com/gyuho/db/raft/raftpb"

// Progress is follower's state in leader's view.
type Progress struct {
	// State is either PROBE, REPLICATE, SNAPSHOT.
	State raftpb.PROGRESS_STATE

	// MatchIndex is the highest known matched entry index
	// of this follower.
	//
	// (etcd raft.Progress.Match)
	MatchIndex uint64

	// NextIndex is the starting index of entries
	// for next replication.
	//
	// (etcd raft.Progress.Next)
	NextIndex uint64

	// ProbePaused is used in PROBE state.
	// When ProbePaused is true, leader stops sending replication messages
	// to this follower.
	//
	// (etcd raft.Progress.Paused)
	ProbePaused bool

	// PendingSnapshotIndex is used in SNAPSHOT state.
	// PendingSnapshotIndex is the index of the ongoing snapshot.
	// When PendingSnapshotIndex is set, leader stops replication
	// to this follower.
	//
	// (etcd raft.Progress.PendingSnapshot)
	PendingSnapshotIndex uint64

	// RecentActive is true if this follower is recently active,
	// such as receiving any message from this follower.
	// It can be reset to false after election timeout.
	//
	// (etcd raft.Progress.RecentActive)
	RecentActive bool

	// inflightState represents the status of buffered messages
	// to this follower. When it's full, no more messages should
	// be sent to this follower.
	inflightState *inflightState
}

// inflightState represents the sliding window of
// inflight messages to this follower. When it's full,
// no more messages should be sent to this follower.
// Whenever leader sends out a message to this follower,
// the index of the last entry in the message should be
// added to inflightState. And when the leader receives
// the response from this follower, it should free the
// previous inflight messages.
//
// (etcd raft.inflights)
type inflightState struct {
	bufferSize    int
	bufferIndexes []uint64

	// starting index in the buffer
	bufferedStartIndex int

	// number of inflights in the buffer
	bufferedCount int
}
