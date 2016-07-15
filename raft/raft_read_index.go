package raft

// LeaderReadState provides the state of read-only query.
// The application must send raftpb.MESSAGE_TYPE_READ_LEADER_CURRENT_COMMITTED_INDEX
// first, before it reads LeaderReadState from NodeReady.
//
// READ_LEADER_CURRENT_COMMITTED_INDEX is used to serve clients' read-only queries without
// going through Raft, but still with 'quorum-get' on. It bypasses the Raft log, but
// still preserves the linearizability of reads, with lower costs.
//
// If a request goes through Raft log, it needs replication, which requires synchronous
// disk writes in order to append those request entries to its log. Since read-only requests
// do not change any state of replicated state machine, these writes can be time- and
// resource-consuming.
//
// To bypass the Raft log with linearizable reads:
//
//   1. If Leader has not yet committed an entry from SenderCurrentTerm, it waits until it has done so.
//
//   2. Leader saves its SenderCurrentCommittedIndex in a local variable 'readIndex', which is used
//      as a lower bound for the version of the state that read-only queries operate against.
//
//   3. Leader must ensure that it hasn't been superseded by a newer Leader,
//      by issuing a new round of heartbeats and waiting for responses from cluster quorum.
//
//   4. These responses from Followers acknowledging the Leader indicates that
//      there was no other Leader at the moment Leader sent out heartbeats.
//
//   5. Therefore, Leader's 'readIndex' was, at the time, the largest committed index,
//      ever seen by any node in the cluster.
//
//   6. Leader now waits for its state machine to advance at least as far as the 'readIndex'.
//      And this is current enought to satisfy linearizability.
//
//   7. Leader can now respond to those read-only client requests.
//
// (Raft 6.4 Processing read-only queries more efficiently, page 72)
//
// (etcd raft.ReadState)
type LeaderReadState struct {
	Index      uint64
	RequestCtx []byte
}
