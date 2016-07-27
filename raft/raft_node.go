package raft

import (
	"fmt"
	"math/rand"
	"sort"

	"github.com/gyuho/db/raft/raftpb"
)

// NoNodeID is a placeholder node ID, only used when there is no leader in the cluster,
// or to reset leader transfer.
const NoNodeID uint64 = 0

// raftNode contains all Raft-algorithm-specific data, wrapping storageRaftLog.
//
// (etcd raft.raft)
type raftNode struct {
	id    uint64
	state raftpb.NODE_STATE

	leaderID      uint64               // (etcd raft.raft.lead)
	allProgresses map[uint64]*Progress // (etcd raft.raft.prs)

	// (etcd raft.raft.raftLog)
	storageRaftLog *storageRaftLog

	rand *rand.Rand

	// electionTimeoutTickNum is the number of ticks for election to time out.
	//
	// (etcd raft.raft.electionTimeout)
	electionTimeoutTickNum int

	// electionTimeoutElapsedTickNum is the number of ticks elapsed
	// since the tick reached the last election timeout.
	//
	// (etcd raft.raft.electionElapsed)
	electionTimeoutElapsedTickNum int

	// randomizedElectionTimeoutTickNum is the random number between
	// [electionTimeoutTickNum, 2 * electionTimeoutTickNum), and gets reset
	// when raftNode state changes to follower or to candidate.
	//
	// (etcd raft.raft.randomizedElectionTimeoutTickNum)
	randomizedElectionTimeoutTickNum int

	// heartbeatTimeoutTickNum is the number of ticks for leader to send heartbeat to its followers.
	//
	// (etcd raft.raft.heartbeatTimeout)
	heartbeatTimeoutTickNum int

	// heartbeatTimeoutElapsedTickNum is the number of ticks elapsed
	// since the tick reached the last heartbeat timeout.
	//
	// (etcd raft.raft.heartbeatElapsed)
	heartbeatTimeoutElapsedTickNum int

	tickFunc func()
	stepFunc func(r *raftNode, msg raftpb.Message)

	// (etcd raft.raft.maxMsgSize)
	maxEntryNumPerMsg uint64

	// (etcd raft.raft.maxInflight)
	maxInflightMsgNum int

	// checkQuorum is true and quorum of cluster is not active for an election timeout,
	// then the leader steps down to follower.
	//
	// (etcd raft.raft.checkQuorum)
	checkQuorum bool

	// (etcd raft.raft.Term)
	currentTerm uint64

	// (etcd raft.raft.Vote)
	votedFor uint64

	// (etcd raft.raft.votes)
	votedFrom map[uint64]bool

	// mailbox contains a slice of messages to be filtered and processed by each step method.
	//
	// (etcd raft.raft.msgs)
	mailbox []raftpb.Message

	// pendingConfigExist is true, then new configuration will be ignored,
	// in preference to the unapplied configuration.
	//
	// (etcd raft.raft.pendingConf)
	pendingConfigExist bool

	// leaderTransfereeID is the ID of the leader transfer target.
	//
	// (Raft §3.10 Leadership transfer extension, p.28)
	//
	// (etcd raft.raft.leadTransferee)
	leaderTransfereeID uint64

	// (etcd raft.raft.readState)
	readState ReadState
}

// newRaftNode creates a new raftNode with the given Config.
func newRaftNode(c *Config) *raftNode {
	if err := c.validate(); err != nil {
		raftLogger.Panicf("invalid raft.Config %v (%+v)", err, c)
	}

	if c.Logger != nil {
		// set the Logger
		raftLogger.SetLogger(c.Logger)
	}
	// otherwise use default logger

	rnd := &raftNode{
		id:    c.ID,
		state: raftpb.NODE_STATE_FOLLOWER, // 0

		leaderID:      NoNodeID,
		allProgresses: make(map[uint64]*Progress),

		storageRaftLog: newStorageRaftLog(c.StorageStable),

		rand: rand.New(rand.NewSource(int64(c.ID))),

		electionTimeoutTickNum:  c.ElectionTickNum,
		heartbeatTimeoutTickNum: c.HeartbeatTimeoutTickNum,

		maxEntryNumPerMsg: c.MaxEntryNumPerMsg,
		maxInflightMsgNum: c.MaxInflightMsgNum,

		checkQuorum: c.CheckQuorum,

		readState: ReadState{Index: uint64(0), RequestCtx: nil},
	}

	hardState, configState, err := c.StorageStable.GetState()
	if err != nil {
		raftLogger.Panicf("newRaftNode c.StorageStable.GetState error (%v)", err)
	}
	if !raftpb.IsEmptyHardState(hardState) {
		rnd.loadHardState(hardState)
	}

	peerIDs := c.allPeerIDs
	if len(configState.IDs) > 0 {
		if len(peerIDs) > 0 {
			raftLogger.Panicf("cannot specify peer IDs both in Config.allPeerIDs(%+v) and configState.IDs(%+v)", peerIDs, configState.IDs)
		}
		// overwrite peerIDs
		peerIDs = configState.IDs
	}

	for _, id := range peerIDs {
		rnd.allProgresses[id] = &Progress{
			NextIndex: 1,
			inflights: newInflights(rnd.maxInflightMsgNum),
		}
	}

	if c.LastAppliedIndex > 0 {
		rnd.storageRaftLog.appliedTo(c.LastAppliedIndex)
	}

	rnd.becomeFollower(rnd.currentTerm, rnd.leaderID)

	var nodeSlice []string
	for _, id := range rnd.allNodeIDs() {
		nodeSlice = append(nodeSlice, fmt.Sprintf("%x", id))
	}

	raftLogger.Infof(`

	NEW NODE %s

`, rnd.describeLong())

	return rnd
}

// sendToMailbox sends a message, given that the requested message
// has already set msg.To for its receiver.
//
// (etcd raft.raft.send)
func (rnd *raftNode) sendToMailbox(msg raftpb.Message) {
	msg.From = rnd.id

	// proposal must go through consensus, which means proposal is to be
	// forwarded to the leader, and replicated back to followers.
	// So it should be treated as local message by setting msg.LogTerm as 0
	if msg.Type != raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER {
		// (X)
		// msg.LogTerm = rnd.currentTerm
		msg.SenderCurrentTerm = rnd.currentTerm
	}

	rnd.mailbox = append(rnd.mailbox, msg)
}

// (etcd raft.raft.quorum)
func (rnd *raftNode) quorum() int {
	return len(rnd.allProgresses)/2 + 1
}

// (etcd raft.raft.resetRandomizedElectionTimeout)
func (rnd *raftNode) randomizeElectionTickTimeout() {
	// [electiontimeout, 2 * electiontimeout)
	rnd.randomizedElectionTimeoutTickNum = rnd.electionTimeoutTickNum + rnd.rand.Intn(rnd.electionTimeoutTickNum)
}

// (etcd raft.raft.pastElectionTimeout)
func (rnd *raftNode) pastElectionTimeout() bool {
	return rnd.electionTimeoutElapsedTickNum >= rnd.randomizedElectionTimeoutTickNum
}

// (etcd raft.raft.abortLeaderTransfer)
func (rnd *raftNode) stopLeaderTransfer() {
	rnd.leaderTransfereeID = NoNodeID
}

// (etcd raft.raft.resetPendingConf)
func (rnd *raftNode) resetPendingConfigExist() {
	rnd.pendingConfigExist = false
}

// (etcd raft.raft.reset)
func (rnd *raftNode) resetWithTerm(term uint64) {
	if rnd.currentTerm != term {
		rnd.currentTerm = term
		rnd.votedFor = NoNodeID
	}

	rnd.leaderID = NoNodeID
	rnd.votedFrom = make(map[uint64]bool)

	rnd.electionTimeoutElapsedTickNum = 0
	rnd.heartbeatTimeoutElapsedTickNum = 0
	rnd.randomizeElectionTickTimeout()

	rnd.stopLeaderTransfer()

	rnd.pendingConfigExist = false

	for id := range rnd.allProgresses {
		rnd.allProgresses[id] = &Progress{
			// NextIndex is the starting index of entries for next replication.
			NextIndex: rnd.storageRaftLog.lastIndex() + 1,
			inflights: newInflights(rnd.maxInflightMsgNum),
		}

		if id == rnd.id {
			// MatchIndex is the highest known matched entry index of this node.
			rnd.allProgresses[id].MatchIndex = rnd.storageRaftLog.lastIndex()
		}
	}
}

// (etcd raft.raft.setProgress)
func (rnd *raftNode) updateProgress(id, matchIndex, nextIndex uint64) {
	rnd.allProgresses[id] = &Progress{
		MatchIndex: matchIndex,
		NextIndex:  nextIndex,
		inflights:  newInflights(rnd.maxInflightMsgNum),
	}
}

// (etcd raft.raft.delProgress)
func (rnd *raftNode) deleteProgress(id uint64) {
	delete(rnd.allProgresses, id)
}

// (etcd raft.raft.softState)
func (rnd *raftNode) softState() *raftpb.SoftState {
	return &raftpb.SoftState{
		NodeState: rnd.state,
		LeaderID:  rnd.leaderID,
	}
}

// (etcd raft.raft.hardState)
func (rnd *raftNode) hardState() raftpb.HardState {
	return raftpb.HardState{
		VotedFor:       rnd.votedFor,
		CommittedIndex: rnd.storageRaftLog.committedIndex,
		Term:           rnd.currentTerm,
	}
}

// (etcd raft.raft.loadState)
func (rnd *raftNode) loadHardState(state raftpb.HardState) {
	if state.CommittedIndex < rnd.storageRaftLog.committedIndex || state.CommittedIndex > rnd.storageRaftLog.lastIndex() {
		raftLogger.Panicf("HardState of %x has committed index %d out of range [%d, %d]",
			rnd.id, state.CommittedIndex, rnd.storageRaftLog.committedIndex, rnd.storageRaftLog.lastIndex())
	}

	rnd.votedFor = state.VotedFor
	rnd.storageRaftLog.committedIndex = state.CommittedIndex
	rnd.currentTerm = state.Term
}

// (etcd raft.raft.nodes)
func (rnd *raftNode) allNodeIDs() []uint64 {
	allNodeIDs := make([]uint64, 0, len(rnd.allProgresses))
	for id := range rnd.allProgresses {
		allNodeIDs = append(allNodeIDs, id)
	}
	sort.Sort(uint64Slice(allNodeIDs))
	return allNodeIDs
}

// (etcd raft.raft.addNode)
func (rnd *raftNode) addNode(id uint64) {
	if _, ok := rnd.allProgresses[id]; ok {
		raftLogger.Infof("%s ignores redundant 'addNode' call to %x (can happen when initial boostrapping entries are applied twice)", rnd.describe(), id)
		return
	}

	matchIndex := uint64(0)
	rnd.updateProgress(id, matchIndex, rnd.storageRaftLog.lastIndex()+1)

	rnd.pendingConfigExist = false
}

// (etcd raft.raft.removeNode)
func (rnd *raftNode) deleteNode(id uint64) {
	rnd.deleteProgress(id)
	rnd.pendingConfigExist = false

	if len(rnd.allProgresses) == 0 {
		raftLogger.Infof("%s has no progresses when raftNode.deleteNode(%x)... returning...", rnd.describe(), id)
		return
	}

	if rnd.state == raftpb.NODE_STATE_LEADER && rnd.leaderTransfereeID == id {
		rnd.stopLeaderTransfer()
	}
}

func (rnd *raftNode) describe() string {
	return fmt.Sprintf("%q %x [term=%d | leader id=%x]", rnd.state, rnd.id, rnd.currentTerm, rnd.leaderID)
}

func (rnd *raftNode) describeLong() string {
	return fmt.Sprintf(`%q %x [node current term=%d | voted for %x | leader id=%x]
	[first log index=%d | committed index=%d | applied index=%d | last log index=%d | last log term=%d]`,
		rnd.state, rnd.id, rnd.currentTerm, rnd.votedFor, rnd.leaderID,
		rnd.storageRaftLog.firstIndex(), rnd.storageRaftLog.committedIndex, rnd.storageRaftLog.appliedIndex,
		rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm())
}

func (rnd *raftNode) assertNodeState(expected raftpb.NODE_STATE) {
	if rnd.state != expected {
		raftLogger.Panicf("%s in unexpected state (expected %q)", rnd.describe(), expected)
	}
}

func (rnd *raftNode) assertUnexpectedNodeState(unexpected raftpb.NODE_STATE) {
	if rnd.state == unexpected {
		raftLogger.Panicf("%s in unexpected state", rnd.describe())
	}
}
