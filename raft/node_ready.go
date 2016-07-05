package raft

import "github.com/gyuho/db/raft/raftpb"

// LeaderReadState provides the state of read-only query.
// The application must send raftpb.LEADER_CURRENT_COMMITTED_INDEX_REQUEST
// first, before it reads LeaderReadState from NodeReady.
//
// (etcd raft.ReadState)
type LeaderReadState struct {
	Index uint64
	Data  []byte
}

// NodeReady represents entries and messages that are ready to read,
// ready to save to stable storage, ready to commit, ready to be
// sent to other peers.
//
// All fields in Ready are read-only.
//
// (etcd raft.Ready)
type NodeReady struct {
	// SoftState provides state that is useful for logging and debugging.
	// The state is volatile and does not need to be persisted to the WAL.
	//
	// SoftState is nil, if there is no update.
	//
	// (etcd raft.Ready.*SoftState)
	SoftState *raftpb.SoftState

	// HardStateToSave is the current state of the Node to be saved in stable storage
	// BEFORE messages are sent out.
	//
	// HardStateToSave is raftpb.EmptyHardState, if there is no update.
	//
	// (etcd raft.Ready.raftpb.HardState)
	HardStateToSave raftpb.HardState

	// SnapshotToSave specifies the Snapshot to save to stable storage.
	//
	// (etcd raft.Ready.Snapshot)
	SnapshotToSave raftpb.Snapshot

	// EntriesToSave specifies the entries to save to stable storage
	// BEFORE messages are sent out.
	//
	// (etcd raft.Ready.Entries)
	EntriesToSave []raftpb.Entry

	// EntriesToCommit specifies the entries to commit, which have already been
	// saved in stable storage.
	//
	// (etcd raft.Ready.CommittedEntries)
	EntriesToCommit []raftpb.Entry

	// MessagesToSend is outbound messages to be sent AFTER EntriesToSave are committed
	// to the stable storage. If it contains raftpb.LEADER_SNAPSHOT_REQUEST, the application
	// MUST report back to Raft when the snapshot has been received or has failed, by calling
	// ReportSnapshot.
	MessagesToSend []raftpb.Message

	// LeaderReadState is updated when Raft receives raftpb.LEADER_CURRENT_COMMITTED_INDEX_REQUEST,
	// only valid for the requested read-request.
	// LeaderReadState is used to serve linearized read-only quorum-get requests without going
	// through Raft log appends, when the Node's applied index is greater than the index in LeaderReadState.
	LeaderReadState LeaderReadState
}

// ContainsUpdates returns true if NodeReady contains any updates.
//
// (etcd raft.Ready.containsUpdates)
func (nr NodeReady) ContainsUpdates() bool {
	return nr.SoftState != nil ||
		!raftpb.IsEmptyHardState(nr.HardStateToSave) ||
		!raftpb.IsEmptySnapshot(nr.SnapshotToSave) ||
		len(nr.EntriesToSave) > 0 ||
		len(nr.EntriesToCommit) > 0 ||
		len(nr.MessagesToSend) > 0 ||
		nr.LeaderReadState.Index != 0
}
