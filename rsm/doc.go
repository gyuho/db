// Package rsm implements replicated state machines.
//
// (etcd etcdserver)
package rsm

/*
WORKING IN PROGRESS

=================================================================================
Package raft implements consensus protocol.

raft consists of:
- `raft.raftpb.Message` struct (LogIndex, LogTerm, Entries, etc.)
- `raft.storageRaftLog` struct (storageUnstable, StorageStable, committedIndex, etc.)
- `raft.raftNode` struct (id, Progress, Step, becomeFollower, etc.)
- `raft.Node` interface (Step, Campaign, Ready, etc.)

raft sends and receives data inside/between raft nodes in `raft.raftpb.Message` format.
Node state is follower, candidate, or leader. And each state has its own step function.
And the step function operates differently for each message type.

Raft leader handles all log replication. All appends are requested in proposal message,
because requests to followers need to be forwarded to leader.

`raft.storageRaftLog` stores Raft logs. When raft appends(maybeAppend) entries to
storage, they are first added to unstable storage, which stores unstable entries
that have not yet been written to `raft.StorageStable`. Leader appends proposals from
followers to its unstable storage, and then commits them updating committed index.
And then leader replicates those logs to followers, in append/snapshot request.

Leader sends snapshot requests when a follower falls behind and leader had already
compacted log entries prior to follower's match index. Follower then receives such
append/snapshot message from leader, and appends to its log (unstable storage first
when it's append request), and then updates its committed index.

`raft.raftNode` contains all Raft-algorithm-specific data, wrapping `raft.storageRaftLog`.
`raft.raftNode` does not implement RPC between nodes. It is just one node in cluster.
It contains configuration data such as heartbeat and election timeout. And states, such
as node state, ID, leader ID, current term, etc..

`raft.raftNode` takes different step functions, depending on node states and message types.
Most algorithms in Raft paper are implemented in step functions.

Leader `raft.raftNode` updates `raft.Progress` that represents log replication status
of followers in leader's viewpoint. `raft.Progress` has inflights in order to rate- and
size-limit the inflight messages in replication.

`raft.Progress` has three states: probe, replicate, snapshot.

`raft.Node` defines Raft node interface to control the lifecyle of Raft node, exposing
only a few interface methods that can be easily used outside of package. `raft.Node` also
wraps StorageStable interface to allow external packages to have their own storage implementation.

The default implementation first creates `raft.raftNode` based on the given configuration,
and launches goroutine that keeps processing raft messages. `raft.Node` keeps sending and
receiving logs via proposal and receiver channel. When those log entries are ready, it sends
them to `ready` channel. And it's application that needs to process this Ready data and makes
progress by calling the `raft.Node.Advance`.

=================================================================================
Package rafthttp ...


=================================================================================
Package raftwal ...


=================================================================================
Package storage ...

mvcc.index interface defines in-memory tree data structure interface with basic Get, Range, Put, Compact, and other methods (index.go)
mvcc.treeIndex struct is B-Tree that satisfies mvcc.index interface (index.go).
mvcc.keyIndex struct defines a node inserted into mvcc.treeIndex (key_index.go).
mvcc.keyIndex struct also keeps track of key versions (key_index.go).

mvcc.KV                    interface defines storage interface with Rev, FirstRev, Range, Put, Compact, TxnBegin, TxnRange, and other methods (kv.go).
mvcc.WatchStream           interface defines watch operations with Watch, Chan, Close, and other methods (watcher.go).
mvcc.Watchable             interface wraps NewWatchStream method that returns mvcc.WatchStream (kv.go).
mvcc.WatchableKV           interface wraps mvcc.KV interface and mvcc.Watchable interface (kv.go).
mvcc.ConsistentWatchableKV interface wraps mvcc.WatchableKV interface with ConsistentIndex method (kv.go).
mvcc.ConsistentIndexGetter interface defines ConsistentIndex method (kvstore.go).

mvcc.store struct embeds mvcc.ConsistentIndexGetter interface, mvcc.index interface, backend.BatchTx interface, lease.Lessor interface and others.
mvcc.NewStore creates a new mvcc.store with a new mvcc.treeIndex for mvcc.index.
mvcc.store struct implements mvcc.KV interface.

=================================================================================
Package mvcc ...


=================================================================================
Package rsm is a replicated state machine built on top of consensus protocol.
It is also an applying machine of those replicated commands.









=================================================================================

etcdserver consists of:
- `etcdserver.Server` interface (Start, Stop, AddMember, etc.)
- `etcdserver.EtcdServer` struct (id, `etcdserver.raftNode`, Storage, applier, etc.)
- `etcdserverpb.rpc.proto`
- `etcdserverpb.api.v3rpc`

`etcdserver.Server` defines typical interface of replicated state machine server.

`etcdserver.EtcdServer` is the default implementation of `etcdserver.Server` interface.

First create a replicated state machine with cluster, Raft configurations: peer addresses,
storage path, WAL path, election timeout, and so on.

`etcdserver.EtcdServer` embeds `etcdserver.raftNode` which then embeds `raft.Node`,
storage, transport, and other Raft states, such as index, term, id, leader id, etc..

`etcdserver.raftNode` runs `raft.Node` interface. When creating a new replicated state machine,
the server starts a `etcdserver.raftNode` that keeps calling `raft.Node` interface methods in
background. `etcdserver.raftNode` keeps receiving from `Ready` channel, and stores, replicates,
applies them.

Then how do client requests get passed to `raft.Node`?

1. etcdserverpb defines RPC interfaces (e.g. KV service interface with Range, Put methods)
2. etcdserverpb then generates KVServer, KVClient interfaces that can en/decode Protocol Buffer messages
3. etcdserver.RaftKV interface defines the same interface as etcdserverpb.KVServer
4. etcdserver.EtcdServer implements etcdserver.RaftKV interface with `etcdserver.raftNode` (v3_server.go)
5. etcdserver.api.v3rpc.kvServer struct embeds this etcdserver.RaftKV interface
6. etcdserver.api.v3rpc.NewKVServer returns a new etcdserverpb.KVServer with etcdserver.EtcdServer
7. etcdserver.api.v3rpc.NewKVServer is used to register gRPC server handler
8. When client sends request with etcdserverpb.KVClient, these handlers are triggered
9. When client sends Range request with etcdserverpb.KVClient, it goes to etcdserver.api.v3rpc.NewKVServer
10. And it goes to etcdserver.api.v3rpc.kvServer.Range
11. And it goes to etcdserver.RaftKV.Range
12. And it goes to etcdserver.EtcdServer.Range
13. If req.Serializable is true, it ranges the local node and returns
14. If req.Serializable is false, it goes to etcdserver.EtcdServer.processInternalRaftRequest
15. And it goes to etcdserver.EtcdServer.processInternalRaftRequestOnce
16. processInternalRaftRequestOnce assigns a unique ID to each request to ensure there is no duplicate request
17. processInternalRaftRequestOnce marshals the request into bytes and proposes that command to the leader
18. Leader's applier goroutine receives this in bytes and Unmarshal it (etcdserver.EtcdServer.applyEntryNormal)
19. And leader applies it and returns

=================================================================================
*/
