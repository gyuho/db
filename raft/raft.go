package raft

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/gyuho/db/raft/raftpb"
)

// NoLeaderNodeID is a placeholder node ID, only used when
// there is no leader in the cluster.
const NoLeaderNodeID uint64 = 0

// Config contains the parameters to start a Raft node.
type Config struct {
	// Logger implements system logging for Raft.
	Logger Logger

	// ID is the id of the Raft node, and 0 when there's no leader.
	// (etcd raft.Config.ID)
	ID uint64

	// ElectionTickNum is the number of ticks between elections.
	// If a follower does not receive any message from a valid leader
	// before ElectionTickNum has elapsed, it becomes a candidate to
	// start an election. ElectionTickNum must be greater than HeartbeatTimeoutTickNum,
	// ideally ElectionTickNum = 10 * HeartbeatTimeoutTickNum.
	// etcd often sets ElectionTickNum as 1 millisecond per tick.
	// (etcd raft.Config.ElectionTick)
	ElectionTickNum int

	// HeartbeatTimeoutTickNum is the number of ticks between heartbeats by a leader.
	// Raft leader must send heartbeat messages to its followers to maintain
	// its leadership.
	// (etcd raft.Config.HeartbeatTick)
	HeartbeatTimeoutTickNum int

	// LeaderCheckQuorum is true, then a leader checks if quorum is active,
	// for an election timeout. If not, the leader steps down.
	LeaderCheckQuorum bool

	// StorageStable implements storage for Raft logs, where a node stores its
	// entries and states, reads the persisted data when needed.
	// Raft node needs to read the previous state and configuration
	// when restarting.
	// (etcd raft.Storage)
	StorageStable StorageStable

	// allPeerIDs contains the IDs of all peers and the node itself.
	// It should only be set when starting a new Raft cluster.
	// (etcd raft.Config.peers)
	allPeerIDs []uint64

	// ---------------------------
	// APPLICATION SPECIFIC CONFIG
	// ---------------------------
	//

	// MaxEntryNumPerMsg is the maximum number of entries for each
	// append message. If 0, it only appends one entry per message.
	// (etcd raft.Config.MaxSizePerMsg)
	MaxEntryNumPerMsg uint64

	// MaxInflightMsgNum is the maximum number of in-flight append messages
	// during optimistic replication phase. Transportation layer usually
	// has its own sending buffer over TCP/UDP. MaxInflighMsgNum is to
	// avoid overflowing that sending buffer.
	// (etcd raft.Config.MaxInflightMsgs)
	MaxInflightMsgNum int

	// LastAppliedIndex is the last applied index of Raft entries.
	// It is only set when restarting a Raft node, so that Raft
	// does not return any entries smaller than or equal to LastAppliedIndex.
	// If LastAppliedIndex is not set when a node restarts, it will return
	// previously applied entries. This is application-specific configuration.
	// (etcd raft.Config.Applied)
	LastAppliedIndex uint64
}

func (c *Config) validate() error {
	if c.StorageStable == nil {
		return errors.New("raft storage cannot be nil")
	}

	if c.ID == NoLeaderNodeID {
		return errors.New("cannot use 0 for node ID")
	}

	if c.HeartbeatTimeoutTickNum <= 0 {
		return errors.New("heartbeat tick must be greater than 0")
	}

	if c.ElectionTickNum <= c.HeartbeatTimeoutTickNum {
		return errors.New("election tick must be greater than heartbeat tick")
	}

	if c.MaxInflightMsgNum <= 0 {
		return errors.New("max number of inflight messages must be greater than 0")
	}

	return nil
}

// raftNode represents Raft-algorithm-specific node.
//
// (etcd raft.raft)
type raftNode struct {
	id    uint64
	state raftpb.NODE_STATE

	leaderID      uint64               // (etcd raft.raft.lead)
	idsToProgress map[uint64]*Progress // (etcd raft.raft.prs)

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

	// leaderIDTransferee is the ID of the leader transfer target
	// when it's not zero (Raft 3.10).
	leaderIDTransferee uint64
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

		leaderID:       NoLeaderNodeID,
		idsToProgress:  make(map[uint64]*Progress),
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
		rnd.idsToProgress[id] = &Progress{
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

func (rnd *raftNode) quorum() int {
	return len(rnd.idsToProgress)/2 + 1
}

func (rnd *raftNode) hasLeader() bool {
	return rnd.leaderID != NoLeaderNodeID
}

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
	allNodes := make([]uint64, 0, len(rnd.idsToProgress))
	for id := range rnd.idsToProgress {
		allNodes = append(allNodes, id)
	}
	sort.Sort(uint64Slice(allNodes))
	return allNodes
}

// (etcd raft.raft.resetRandomizedElectionTimeout)
func (rnd *raftNode) RandomizeElectionTickTimeout() {
	rnd.randomizedElectionTimeoutTickNum = rnd.electionTimeoutTickNum + rnd.rand.Intn(rnd.electionTimeoutTickNum)
}

// sendToMailbox sends a message, given that the requested message
// has already set msg.To for its receiver.
//
// (etcd raft.raft.send)
func (rnd *raftNode) sendToMailbox(msg raftpb.Message) {
	msg.From = rnd.id

	// proposal must go through consensus, which means
	// proposal is to be forwarded to the leader,
	// and replicated back to followers.
	// so it should be treated as local message
	// by setting msg.LogTerm as 0
	if msg.Type != raftpb.MESSAGE_TYPE_PROPOSAL {
		msg.LogTerm = rnd.term
	}

	rnd.mailbox = append(rnd.mailbox, msg)
}

func (rnd *raftNode) becomeFollower(term, leaderID uint64) {

}

// Peer contains peer ID and context data.
//
// (etcd raft.Peer)
type Peer struct {
	ID   uint64
	Data []byte
}
