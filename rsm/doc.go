// Package rsm implements replicated state machine.
//
// (etcd etcdserver)
package rsm

// WORKING IN PROGRESS
/*

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
Package raftsnap ...


=================================================================================
Package kvstorage ...


=================================================================================
Package mvcc ...


=================================================================================
Package rsm is a replicated state machine built on top of consensus protocol.
Think of it as applying machine of replicated commands.

rsm consists of:
- `etcdserver.Server` interface (Start, Stop, AddMember, etc.)
- `etcdserver.EtcdServer` struct (id, `etcdserver.raftNode`, Storage, applier, etc.)
- `etcdserverpb.rpc.proto`
- `etcdserverpb.api.v3rpc`

`etcdserver.Server` lays out the typical interfaces of replicated state machine server.

`etcdserver.EtcdServer` is the default implementation of `etcdserver.Server` interface.

First create a replicated state machine with cluster, Raft configurations: peer addresses,
storage path, WAL path, election timeout, and so on.

`etcdserver.EtcdServer` embeds `etcdserver.raftNode` which then embeds `raft.Node`,
storage, transport, and other Raft states, such as index, term, id, leader id, etc..

`etcdserver.raftNode` is the consumer of `raft.Node` interface. When creating a
new replicated state machine, the server also starts a `etcdserver.raftNode` that
keeps calling `raft.Node` interface methods in the background. `etcdserver.raftNode`
is waiting to receive from `Ready` channel, and stores, replicates, applies them.

Then how do client requests get passed to `raft.Node`?

`etcdserver.EtcdServer` has `processInternalRaftRequest` method to handle all client
requests, which calls `processInternalRaftRequestOnce` that proposes client requests
to the leader. Each request is given a unique id so that there is no duplicate request.

For example, when server receives client Range request, it calls `etcdserver.EtcdServer.Range`
and marshals the request into bytes to propose that command to the leader.

`processInternalRaftRequest` takes context and `InternalRaftRequest` as arguments.
And `InternalRaftRequest` contains all types of request, and the request tells the
applier which request to apply.

Then how does the proposed message turn into `InternalRaftRequest`?
When proposal is received over network, the server calls `applyEntryNormal`,
and unmarshals the data into `pb.InternalRaftRequest`, and calls `Apply` method
on that request.

If range request is serializable, it is not forwarded to leader, only ranging the
local node without going through consensus protocol.

`etcdserverpb.rpc.proto` defines rpc interfaces. For example, it defines `KVClient` with
`Range` method, with `RangeRequest` and `RangeResponse`. It just registers the protocol
buffer type with marshallers and unmarshallers.

`etcdserverpb.api.v3rpc` implements these rpc interfaces, by calling `Range` method
implemented in `etcdserver.EtcdServer`. So if client requests `Range` request to this
gRPC server, it calls `etcdserver.EtcdServer.Range`.

=================================================================================
*/
