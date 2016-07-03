package walpb

const (
	// RECORD_TYPE_METADATA contains the metadata of the record.
	RECORD_TYPE_METADATA int64 = iota + 1

	// RECORD_TYPE_ENTRY is log entries to be stored in stable storage
	// before messages are sent.
	//
	// (etcd: raft.raftpb.Entry)
	RECORD_TYPE_ENTRY

	// RECORD_TYPE_HARDSTATE is the current state of the node.
	// It is stored in stable storage before messages are sent.
	//
	// (etcd: raft.raftpb.HardState)
	RECORD_TYPE_HARDSTATE

	// RECORD_TYPE_CRC is the CRC hash value of record.
	RECORD_TYPE_CRC

	// RECORD_TYPE_SNAPSHOT is the snapshot to be stored in stable storage
	// before messages are sent.
	//
	// (etcd: raft.raftpb.Snapshot)
	RECORD_TYPE_SNAPSHOT
)
