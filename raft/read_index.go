package raft

import "github.com/gyuho/db/raft/raftpb"

// ReadState provides the state of read-only query.
// The application must send MESSAGE_TYPE_TRIGGER_READ_INDEX first,
// before it reads ReadState from Ready.
//
// READ_INDEX is used to serve clients' read-only queries without
// going through Raft, but still with 'quorum-get' on. It bypasses the Raft log, but
// still preserves the linearizability of reads, with lower costs.
//
// If a request goes through Raft log, it needs replication, which requires synchronous
// disk writes in order to append those request entries to its log. Since read-only requests
// do not change any state of replicated state machine, these writes can be time- and
// resource-consuming.
//
//
// (Raft §6.4 Processing read-only queries more efficiently, p.72)
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
//
// - Leader records current commit index in readIndex
// - Leader sends readIndex to followers
// - For followers, readIndex is the largest commit index ever seen by any server
// - Read-request within readIndex is now served locally with linearizability
// - More efficient, avoids synchronous disk writes
//
//
// (etcd raft.ReadState)
type ReadState struct {
	Index      uint64
	RequestCtx []byte
}

// ReadOnlyOption specifies how the read only request is processed.
//
// (etcd raft.ReadOnlyOption)
type ReadOnlyOption int

const (
	// ReadOnlySafe guarantees the linearizability of the read only request by
	// communicating with the quorum. It is the default and suggested option.
	ReadOnlySafe ReadOnlyOption = iota

	// ReadOnlyLeaseBased ensures linearizability of the read only request by
	// relying on the leader lease. It can be affected by clock drift.
	ReadOnlyLeaseBased
)

// (etcd raft.readIndexStatus)
type readIndexStatus struct {
	req      raftpb.Message
	index    uint64
	ackCount int
}

// (etcd raft.readOnly)
type readOnly struct {
	option           ReadOnlyOption
	pendingReadIndex map[string]*readIndexStatus
	readIndexQueue   []string
}

// (etcd raft.newReadOnly)
func newReadOnly(option ReadOnlyOption) *readOnly {
	return &readOnly{
		option:           option,
		pendingReadIndex: make(map[string]*readIndexStatus),
	}
}

// (etcd raft.readOnly.addRequest)
func (ro *readOnly) addRequest(msg raftpb.Message, index uint64) {
	ctx := string(msg.Entries[0].Data)
	if _, ok := ro.pendingReadIndex[ctx]; ok {
		return
	}
	ro.pendingReadIndex[ctx] = &readIndexStatus{req: msg, index: index, ackCount: 1}
	ro.readIndexQueue = append(ro.readIndexQueue, ctx)
}

// (etcd raft.readOnly.recvAck)
func (ro *readOnly) recvAck(msg raftpb.Message) int {
	rs, ok := ro.pendingReadIndex[string(msg.Context)]
	if !ok {
		return 0
	}
	rs.ackCount++
	return rs.ackCount
}

// (etcd raft.readOnly.advance)
func (ro *readOnly) advance(msg raftpb.Message) []*readIndexStatus {
	var rss []*readIndexStatus
	var i int
	ctx := string(msg.Context)

	for _, okctx := range ro.readIndexQueue {
		i++
		rs, ok := ro.pendingReadIndex[okctx]
		if !ok {
			panic("cannot find corresponding read state from pending map")
		}
		rss = append(rss, rs)
		delete(ro.pendingReadIndex, okctx)
		if okctx == ctx {
			break
		}
	}
	ro.readIndexQueue = ro.readIndexQueue[i:]
	return rss
}
