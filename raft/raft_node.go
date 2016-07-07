package raft

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/gyuho/db/raft/raftpb"
)

// NoneNodeID is a placeholder node ID, only used when there is no leader in the cluster,
// or to reset leader transfer.
const NoneNodeID uint64 = 0

// raftNode represents Raft-algorithm-specific node.
//
// (etcd raft.raft)
type raftNode struct {
	id    uint64
	state raftpb.NODE_STATE

	leaderID      uint64                       // (etcd raft.raft.lead)
	allProgresses map[uint64]*FollowerProgress // (etcd raft.raft.prs)

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
	// [electionTimeoutTickNum, 2 * electionTimeoutTickNum - 1], and gets reset
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

	maxEntryNumPerMsg uint64
	maxInflightMsgNum int

	leaderCheckQuorum bool

	term      uint64          // (etcd raft.raft.Term)
	votedFor  uint64          // (etcd raft.raft.Vote)
	votedFrom map[uint64]bool // (etcd raft.raft.votes)

	// mailbox contains a slice of messages to be filtered and processed
	// by each step method.
	mailbox []raftpb.Message

	// pendingConfigExist is true, then new configuration will be ignored,
	// in preference to the unapplied configuration.
	pendingConfigExist bool

	// leaderTransfereeID is the ID of the leader transfer target
	// when it's not zero (Raft 3.10).
	leaderTransfereeID uint64
}

// newRaftNode creates a new raftNode with the given Config.
func newRaftNode(c *Config) *raftNode {
	if err := c.validate(); err != nil {
		raftLogger.Panicf("invalid raft.Config %+v (%v)", c, err)
	}

	if c.Logger != nil { // set the Logger
		raftLogger.SetLogger(c.Logger)
	}

	rnd := &raftNode{
		id:    c.ID,
		state: raftpb.NODE_STATE_FOLLOWER, // 0

		leaderID:       NoneNodeID,
		allProgresses:  make(map[uint64]*FollowerProgress),
		storageRaftLog: newStorageRaftLog(c.StorageStable),

		rand: rand.New(rand.NewSource(int64(c.ID))),

		electionTimeoutTickNum:  c.ElectionTickNum,
		heartbeatTimeoutTickNum: c.HeartbeatTimeoutTickNum,

		maxEntryNumPerMsg: c.MaxEntryNumPerMsg,
		maxInflightMsgNum: c.MaxInflightMsgNum,

		leaderCheckQuorum: c.LeaderCheckQuorum,
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
			raftLogger.Panicf("cannot specify peer IDs both in Config.allPeerIDs and configState.IDs")
		}
		// overwrite peerIDs
		peerIDs = configState.IDs
	}
	for _, id := range peerIDs {
		rnd.allProgresses[id] = &FollowerProgress{
			NextIndex: 1,
			inflights: newInflights(rnd.maxInflightMsgNum),
		}
	}

	if c.LastAppliedIndex > 0 {
		rnd.storageRaftLog.appliedTo(c.LastAppliedIndex)
	}

	rnd.becomeFollower(rnd.term, rnd.leaderID)

	var nodeSlice []string
	for _, id := range rnd.allNodes() {
		nodeSlice = append(nodeSlice, fmt.Sprintf("%x", id))
	}

	raftLogger.Infof(`

newRaftNode

    state = %q
       id = %x
all nodes = %q

first index = %d
last  index = %d

     term = %d
last term = %d

committed index = %d
applied   index = %d


`, rnd.state, rnd.id, strings.Join(nodeSlice, ", "),
		rnd.storageRaftLog.firstIndex(), rnd.storageRaftLog.lastIndex(),
		rnd.term, rnd.storageRaftLog.lastTerm(),
		rnd.storageRaftLog.committedIndex, rnd.storageRaftLog.appliedIndex,
	)

	return rnd
}

// (etcd raft.raft.loadState)
func (rnd *raftNode) loadHardState(state raftpb.HardState) {
	if state.CommittedIndex < rnd.storageRaftLog.committedIndex || state.CommittedIndex > rnd.storageRaftLog.lastIndex() {
		raftLogger.Panicf("HardState of %x has committed index %d out of range [%d, %d]",
			rnd.id, state.CommittedIndex, rnd.storageRaftLog.committedIndex, rnd.storageRaftLog.lastIndex())
	}

	rnd.votedFor = state.VotedFor
	rnd.storageRaftLog.committedIndex = state.CommittedIndex
	rnd.term = state.Term
}

// (etcd raft.raft.nodes)
func (rnd *raftNode) allNodes() []uint64 {
	allNodes := make([]uint64, 0, len(rnd.allProgresses))
	for id := range rnd.allProgresses {
		allNodes = append(allNodes, id)
	}
	sort.Sort(uint64Slice(allNodes))
	return allNodes
}

// (etcd raft.raft.resetRandomizedElectionTimeout)
func (rnd *raftNode) randomizeElectionTickTimeout() {
	rnd.randomizedElectionTimeoutTickNum = rnd.electionTimeoutTickNum + rnd.rand.Intn(rnd.electionTimeoutTickNum)
}

// (etcd raft.raft.quorum)
func (rnd *raftNode) quorum() int {
	return len(rnd.allProgresses)/2 + 1
}

// checkQuorumActive returns true if the quorum of the cluster
// is active in the view of the local raft state machine.
//
// (etcd raft.raft.checkQuorumActive)
func (rnd *raftNode) checkQuorumActive() bool {
	activeN := 0
	for id := range rnd.allProgresses {
		if id == rnd.id {
			activeN++ // self is always active
			continue
		}

		if rnd.allProgresses[id].RecentActive {
			activeN++
		}

		// and resets the RecentActive
		rnd.allProgresses[id].RecentActive = false
	}

	return activeN >= rnd.quorum()
}

// (etcd raft.raft.abortLeaderTransfer)
func (rnd *raftNode) stopLeaderTransfer() {
	rnd.leaderTransfereeID = NoneNodeID
}

// promotable return true if the local state machine can be promoted to leader.
//
// (etcd raft.raft.promotable)
func (rnd *raftNode) promotable() bool {
	_, ok := rnd.allProgresses[rnd.id]
	return ok
}
