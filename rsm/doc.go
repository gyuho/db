// Package rsm implements replicated state machine.
//
// (etcd etcdserver)
package rsm

// WORKING IN PROGRESS
/*

=================================================================================
Package raft implements consensus protocol.

raft consists of:
- 'Message' struct (LogIndex, LogTerm, Entries, etc.)
- 'storageRaftLog' struct (storageUnstable, StorageStable, committedIndex, etc.)
- 'raftNode' struct (id, Progress, Step, becomeFollower, etc.)
- 'Node' interface (Step, Campaign, Ready, etc.)

Message is used to send and receive data inside/between raft nodes. Each node state
defines its step function to behave based on Message.Type. Raft uses leader to handle
all log replication. Therefore, all append requests are sent as a proposal, so that
it can be forwarded to the leader.

storageRaftLog stores Raft logs. When raft appends(maybeAppend) entries to storage,
they are first added to unstable storage, which stores unstable entries that have
not yet been written to the StorageStable. So when leader receives proposal from
followers, it first appends those entries to its unstable storage. And then updates
its committed index. And then replicates leader's log either with append request
or snapshot request. Leader sends snapshot requests when a follower falls behind
to a point that leader had already compacted log entries prior to follower's match
index. Follower then receives such append/snapshot message from leader, and appends
to its log (unstable storage first when it's append request), and then updates its
commit index.

raftNode contains all Raft-algorithm-specific data, wrapping storageRaftLog.
raftNode does not implement RPC or message transportation between nodes.
raftNode is just one node in Raft cluster. It contains configuration data such as heartbeat
and election timeout. It contains information such as node state, ID, leader ID, current
term, etc.. raftNode takes different step functions, depending on node state and message
type. Each step function implements most of the algorithms in Raft paper.

If raftNode is leader, it also updates log replication Progress, which
contains useful information about log replication status from leader to follower.
Progress has inflights to rate,size-limit inflight messages in replication.
Progress has three states: probe, replicate, snapshot.

Node defines the interface of a node in Raft cluster. Node still does not implement
RPC or transport. It creates raftNode based on the given configuration, processes node
step functions in the background, and allows only limited method calls from external packages.
Any package that satisfies Node interface can use raft package without worrying about
Raft consensus algorithm details. Node also wraps StorageStable interface, so that external
packages can have their own storage implementation.

Node keeps sending and receiving logs via proposal, receiver channel. And once those log entries
are ready, it sends them to ready channel. And it expects the application to process this
Ready data, and make progress by calling Advance method.


=================================================================================
Package rafthttp ...


=================================================================================
Package raftwal ...


=================================================================================
Package raftsnap ...


=================================================================================
Package kvstorage ...


=================================================================================
Package mvcc ...


=================================================================================
Package rsm is a replicated state machine built on top of consensus protocol.
Think of it as applying machine of replicated commands.

rsm consists of:
- 'Server' (id, raftNode, Storage, etc.)
- 'raftNode'
- 'rsmpb'
- 'api'

=================================================================================
*/
