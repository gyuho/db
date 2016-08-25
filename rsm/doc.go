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

Only leader can send snapshots.

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

`raft.Ready` returns a channel that receives point-in-time state of `raft.Node`.
`raft.Advance` method MUST be followed, after applying the state in `raft.Ready`.
Application cannot receive `raft.Ready` again, without calling `raft.Advance`.
Application must commit `raft.Ready` entries to storage before calling `raft.Advance`.
Applying log entires can be asynchronous (etcd does this in scheduler in the background).

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
Package rsm is a replicated state machine built on top of consensus protocol.
It is also an applying machine of those replicated commands.

etcdserver.NewServer creates a new etcdserver.EtcdServer.
 1. mkdir cfg.DataDir
    - MemberDir   = DataDir/member
    - WALDir      = MemberDir/wal
    - SnapDir     = MemberDir/snap
    - BackendPath = SnapDir/db
 2. if WALExist, newCluster
    - setBackend
    - startNode
 3. if WALExist, !newCluster
    - setBackend
    - restartNode
 4. start EtcdServer
 5. raftNode.start
 6. for-loop keeps receiving from raft.Ready channel
 7. sends Ready to applyc
 8. receive Ready from applyc
 9. EtcdServer applies these Ready
10. If Ready.Snapshot is not empty, save,apply Snapshot
11. r.Advance


(etcdserver/raft.go)
func (r *raftNode) start(s *EtcdServer) {
    case rd := <-r.Ready():
        ap := apply{
            entries:  rd.CommittedEntries,
            snapshot: rd.Snapshot,
            raftDone: raftDone,
        }
        r.applyc <- ap
        r.s.send(rd.Messages) // if leader
        r.storage.Save(rd.HardState, rd.Entries)

        if !raft.IsEmptySnap(rd.Snapshot)
            r.storage.SaveSnap(rd.Snapshot)
            r.raftStorage.ApplySnapshot(rd.Snapshot)

        r.raftStorage.Append(rd.Entries)
        raftDone <- struct{}{}
        r.Advance()

(etcdserver/server.go)
func (s *EtcdServer) send(ms []raftpb.Message) {
    if ms[i].Type == raftpb.MsgSnap {
        s.msgSnapC <- ms[i]:

(etcdserver/server.go)
func (s *EtcdServer) run() {
    for {
        select {
            case ap := <-s.r.applyc:
                s.applyAll(&ep, &ap)

(etcdserver/server.go)
func (s *EtcdServer) applyAll(ep *etcdProgress, apply *apply) {
    s.applySnapshot(ep, apply)
    s.applyEntries(ep, apply)
    <-apply.raftDone
    s.triggerSnapshot(ep)
    m := <-s.msgSnapC
        merged := s.createMergedSnapshotMessage(m, ep.appliedi, ep.confState)
        s.sendMergedSnap(merged)

(etcdserver/server.go)
func (s *EtcdServer) triggerSnapshot(ep *etcdProgress) {
    if ep.appliedi-ep.snapi > s.snapCount
        s.snapshot(snapi uint64, confState raftpb.ConfState)

(etcdserver/server.go)
func (s *EtcdServer) snapshot(snapi uint64, confState raftpb.ConfState) {
    d, err := clone.SaveNoCopy()
    snap, err := s.r.raftStorage.CreateSnapshot(snapi, &confState, d)
    s.KV().Commit()
    r.storage.SaveSnap(snap)
    r.raftStorage.Compact(compacti) // discard all entries before compacti (now only entries "compacti ≤" are left)

(raft/raft.go)
func (r *raft) sendAppend(to uint64) {
    term, errt := r.raftLog.term(pr.Next - 1)
    ents, erre := r.raftLog.entries(pr.Next, r.maxMsgSize)
    if errt != nil || erre != nil { // send snapshot if we failed to get term or entries
        m.Type = pb.MsgSnap
        snapshot, err := r.raftLog.snapshot()
        m.Snapshot = snapshot
        pr.becomeSnapshot(sindex)

(raft/raft.go)
func stepCandidate(r *raft, m pb.Message)
func stepFollower(r *raft, m pb.Message) {
    case pb.MsgSnap:
        r.electionElapsed = 0
        r.lead = m.From
        r.handleSnapshot(m)

func (r *raft) handleSnapshot(m pb.Message) {
    sindex, sterm := m.Snapshot.Metadata.Index, m.Snapshot.Metadata.Term
    if r.restore(m.Snapshot) {

func (r *raft) restore(s pb.Snapshot) bool {
    r.raftLog.restore(s)

(raft/log.go)
func (l *raftLog) restore(s pb.Snapshot) {
    l.unstable.restore(s)

(raft/log_unstable.go)
func (u *unstable) restore(s pb.Snapshot) {
    u.snapshot = &s


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

etcdctl put foo bar

# client side
-> etcdserverpb.KVClient.Put
-> etcdserverpb (c *kVClient) Put: key:"foo" value:"bar"

# server side
etcdserverpb._KV_Put_Handler
-> etcdserver.api.v3rpc (s *kvServer) Put
-> etcdserver.EtcdServer.Put(ctx context.Context, r *pb.PutRequest)
-> etcdserver.EtcdServer.processInternalRaftRequest
-> etcdserver.EtcdServer.processInternalRaftRequestOnce
-> data, err := r.Marshal()
-> etcdserver.EtcdServer.raftNode.Propose(cctx, data)
-> etcdserver.EtcdServer.raftNode.Ready()               (raft.go)
-> f := func(context.Context) { s.applyAll(&ep, &ap) }  (server.go)

(s *EtcdServer) applyAll
s.applyEntries(ep, apply)
s.apply(ents, &ep.confState)
s.applyEntryNormal(&e)
s.applyV3.Apply(&raftReq)
Put

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
