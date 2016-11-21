package raft

import (
	"errors"
	"fmt"
	"sort"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
)

// NoNodeID is a placeholder node ID, only used when there is no leader in the cluster,
// or to reset leader transfer.
const NoNodeID uint64 = 0

// Config contains the parameters to start a Raft node.
type Config struct {
	// allPeerIDs contains the IDs of all peers and the node itself.
	// It should only be set when starting a new Raft cluster.
	//
	// (etcd raft.Config.peers)
	allPeerIDs []uint64

	// ID is the id of the Raft node, and 0 when there's no leader.
	//
	// (etcd raft.Config.ID)
	ID uint64

	// ElectionTickNum is the number of ticks between elections.
	// If a follower does not receive any message from a valid leader
	// before ElectionTickNum has elapsed, it becomes a candidate to
	// start an election. ElectionTickNum must be greater than HeartbeatTimeoutTickNum,
	// ideally ElectionTickNum = 10 * HeartbeatTimeoutTickNum.
	// etcd often sets ElectionTickNum as 1 millisecond per tick.
	//
	// (etcd raft.Config.ElectionTick)
	ElectionTickNum int

	// HeartbeatTimeoutTickNum is the number of ticks between heartbeats by a leader.
	// Raft leader must send heartbeat messages to its followers to maintain
	// its leadership.
	//
	// (etcd raft.Config.HeartbeatTick)
	HeartbeatTimeoutTickNum int

	// CheckQuorum is true, then a leader checks if quorum is active,
	// for an election timeout. If not, the leader steps down.
	//
	// (etcd raft.Config.CheckQuorum)
	CheckQuorum bool

	// PreVote enables the Pre-Vote algorithm described in raft thesis section
	// 9.6. This prevents disruption when a node that has been partitioned away
	// rejoins the cluster.
	//
	//
	// (Raft §9.6 Preventing disruptions when a server rejoins the cluster, p.136)
	//
	// In the Pre-Vote algorithm, a candidate only increments its term
	// if it first learns from a majority of the cluster that they would be willing
	// to grant the candidate their votes (if the candidate’s log is sufficiently
	// up-to-date, and the voters have not received heartbeats from a valid leader
	// for at least a baseline election timeout).
	//
	// No node increases its term number unless the pre-election indicates that the
	// campaigining node would win.	This minimizes disruption when a partitioned node
	// rejoins the cluster.
	//
	// (etcd raft.Config.PreVote)
	PreVote bool

	// StorageStable implements storage for Raft logs, where a node stores its
	// entries and states, reads the persisted data when needed.
	// Raft node needs to read the previous state and configuration
	// when restarting.
	//
	// (etcd raft.Storage)
	StorageStable StorageStable

	// ---------------------------
	// APPLICATION SPECIFIC CONFIG
	// ---------------------------
	//

	// MaxEntryNumPerMsg is the maximum number of entries for each
	// append message. If 0, it only appends one entry per message.
	//
	// (etcd raft.Config.MaxSizePerMsg)
	MaxEntryNumPerMsg uint64

	// MaxInflightMsgNum is the maximum number of in-flight append messages
	// during optimistic replication phase. Transportation layer usually
	// has its own sending buffer over TCP/UDP. MaxInflighMsgNum is to
	// avoid overflowing that sending buffer.
	//
	// (etcd raft.Config.MaxInflightMsgs)
	MaxInflightMsgNum int

	// LastAppliedIndex is the last applied index of Raft entries.
	// It is only set when restarting a Raft node, so that Raft
	// does not return any entries smaller than or equal to LastAppliedIndex.
	// If LastAppliedIndex is not set when a node restarts, it will return
	// previously applied entries. This is application-specific configuration.
	//
	// (etcd raft.Config.Applied)
	LastAppliedIndex uint64

	// ReadOnlyOption specifies how the read only request is processed.
	//
	// (etcd raft.Config.ReadOnlyOption)
	ReadOnlyOption ReadOnlyOption

	// Logger implements system logging for Raft.
	Logger Logger
}

func (c *Config) validate() error {
	if c.StorageStable == nil {
		return errors.New("raft storage cannot be nil")
	}

	if c.ID == NoNodeID {
		return errors.New("cannot use 0 for node ID")
	}

	if c.HeartbeatTimeoutTickNum <= 0 {
		return fmt.Errorf("heartbeat tick (%d) must be greater than 0", c.HeartbeatTimeoutTickNum)
	}

	if c.ElectionTickNum <= c.HeartbeatTimeoutTickNum {
		return fmt.Errorf("election tick (%d) must be greater than heartbeat tick (%d)", c.ElectionTickNum, c.HeartbeatTimeoutTickNum)
	}

	if c.MaxInflightMsgNum <= 0 {
		return errors.New("max number of inflight messages must be greater than 0")
	}

	return nil
}

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
	preVote     bool

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

	readOnly   *readOnly   // (etcd raft.raft.readOnly)
	readStates []ReadState // (etcd raft.raft.readStates)
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

		electionTimeoutTickNum:  c.ElectionTickNum,
		heartbeatTimeoutTickNum: c.HeartbeatTimeoutTickNum,

		maxEntryNumPerMsg: c.MaxEntryNumPerMsg,
		maxInflightMsgNum: c.MaxInflightMsgNum,

		checkQuorum: c.CheckQuorum,
		preVote:     c.PreVote,

		readOnly: newReadOnly(c.ReadOnlyOption),
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

	raftLogger.Infof("NEW NODE %s", rnd.describe())
	return rnd
}

// sendToMailbox sends a message, given that the requested message
// has already set msg.To for its receiver.
//
// (etcd raft.raft.send)
func (rnd *raftNode) sendToMailbox(msg raftpb.Message) {
	msg.From = rnd.id

	if msg.Type == raftpb.MESSAGE_TYPE_PRE_CANDIDATE_REQUEST_VOTE || msg.Type == raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE {
		if msg.SenderCurrentTerm == 0 {
			// PreVote RPCs are sent at a term other than our actual term, so the code
			// that sends these messages is responsible for setting the term.
			panic(fmt.Sprintf("term should be set when sending %s", msg.Type))
		}
	} else {
		if msg.SenderCurrentTerm != 0 {
			panic(fmt.Sprintf("term should not be set when sending %s (was %d)", msg.Type, msg.SenderCurrentTerm))
		}

		// proposal must go through consensus, which means proposal is to be
		// forwarded to the leader, and replicated back to followers.
		// So it should be treated as local message by setting msg.LogTerm as 0
		//
		// readIndex request is also forwarded to leader
		if msg.Type != raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER && msg.Type != raftpb.MESSAGE_TYPE_TRIGGER_READ_INDEX {
			// (X)
			// msg.LogTerm = rnd.currentTerm
			msg.SenderCurrentTerm = rnd.currentTerm
		}
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
	rnd.randomizedElectionTimeoutTickNum = rnd.electionTimeoutTickNum + globalRand.Intn(rnd.electionTimeoutTickNum)
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

	rnd.pendingConfigExist = false
	rnd.readOnly = newReadOnly(rnd.readOnly.option)
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
	rnd.pendingConfigExist = false

	if _, ok := rnd.allProgresses[id]; ok {
		raftLogger.Infof("%s ignores redundant 'addNode' call to %s (can happen when initial boostrapping entries are applied twice)", rnd.describe(), types.ID(id))
		return
	}

	matchIndex := uint64(0)
	rnd.updateProgress(id, matchIndex, rnd.storageRaftLog.lastIndex()+1)
}

// (etcd raft.raft.removeNode)
func (rnd *raftNode) deleteNode(id uint64) {
	rnd.deleteProgress(id)
	rnd.pendingConfigExist = false

	if len(rnd.allProgresses) == 0 {
		raftLogger.Infof("%s has no progresses when raftNode.deleteNode(%s); returning", rnd.describe(), types.ID(id))
		return
	}

	if rnd.state == raftpb.NODE_STATE_LEADER && rnd.leaderTransfereeID == id {
		rnd.stopLeaderTransfer()
	}
}

func (rnd *raftNode) describe() string {
	return fmt.Sprintf("%q %s [term=%d | leader=%s]", rnd.state, types.ID(rnd.id), rnd.currentTerm, types.ID(rnd.leaderID))
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

// setRandomizedElectionTimeoutTickNum set up the value by caller instead of choosing
// by system, in some test scenario we need to fill in some expected value to
// ensure the certainty.
//
// (etcd raft.setRandomizedElectionTimeout)
func (rnd *raftNode) setRandomizedElectionTimeoutTickNum(num int) {
	rnd.randomizedElectionTimeoutTickNum = num
}
