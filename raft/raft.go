package raft

import (
	"errors"
	"math/rand"

	"github.com/gyuho/db/raft/raftpb"
)

// NoLeaderNodeID is a placeholder node ID, only used when
// there is no leader in the cluster.
const NoLeaderNodeID uint64 = 0

// Config contains the parameters to start a Raft node.
type Config struct {
	// ID is the id of the Raft node, and 0 when there's no leader.
	// (etcd raft.Config.ID)
	ID uint64

	// allNodeIDs contains the IDs of all peers and the node itself.
	// It should only be set when starting a new Raft cluster.
	// (etcd raft.Config.peers)
	allNodeIDs []uint64

	// Logger implements system logging for Raft.
	Logger Logger

	// StableStorage implements storage for Raft logs, where a node stores its
	// entries and states, reads the persisted data when needed.
	// Raft node needs to read the previous state and configuration
	// when restarting.
	// (etcd raft.Storage)
	StableStorage StableStorage

	// ElectionTick is the number of ticks between elections.
	// If a follower does not receive any message from a valid leader
	// before ElectionTick has elapsed, it becomes a candidate to
	// start an election. ElectionTick must be greater than HeartbeatTick,
	// ideally ElectionTick = 10 * HeartbeatTick.
	// etcd often sets ElectionTick as 1 millisecond per tick.
	// (etcd raft.Config.ElectionTick)
	ElectionTick int

	// HeartbeatTick is the number of ticks between heartbeats by a leader.
	// Raft leader must send heartbeat messages to its followers to maintain
	// its leadership.
	// (etcd raft.Config.HeartbeatTick)
	HeartbeatTick int

	// LeaderCheckQuorum is true, then a leader checks if quorum is active,
	// for an election timeout. If not, the leader steps down.
	LeaderCheckQuorum bool

	// ---------------------------
	// APPLICATION SPECIFIC CONFIG
	// ---------------------------
	//

	// LastAppliedIndex is the last applied index of Raft entries.
	// It is only set when restarting a Raft node, so that Raft
	// does not return any entries smaller than or equal to LastAppliedIndex.
	// If LastAppliedIndex is not set when a node restarts, it will return
	// previously applied entries. This is application-specific configuration.
	// (etcd raft.Config.Applied)
	LastAppliedIndex uint64

	// MaxEntryPerMsg is the maximum number of entries for each
	// append message. If 0, it only appends one entry per message.
	// (etcd raft.Config.MaxSizePerMsg)
	MaxEntryPerMsg uint64

	// MaxInflightMsgs is the maximum number of in-flight append messages
	// during optimistic replication phase. Transportation layer usually
	// has its own sending buffer over TCP/UDP. MaxInflighMsgNum is to
	// avoid overflowing that sending buffer.
	// (etcd raft.Config.MaxInflightMsgs)
	MaxInflightMsgs int
}

func (c *Config) validate() error {
	if c.ID == NoLeaderNodeID {
		return errors.New("cannot use 0 for node ID")
	}

	if c.Logger == nil {
		return errors.New("logger cannot be nil")
	}

	if c.raftStorage == nil {
		return errors.New("raft log storage cannot be nil")
	}

	if c.HeartbeatTick <= 0 {
		return errors.New("heartbeat tick must be greater than 0")
	}

	if c.ElectionTick <= c.HeartbeatTick {
		return errors.New("election tick must be greater than heartbeat tick")
	}

	if c.MaxInflightMsgs <= 0 {
		return errors.New("max number of inflight messages must be greater than 0")
	}

	return nil
}

// raftNode represents Raft-algorithm-specific node.
// (etcd raft.raft)
type raftNode struct {
	id    uint64
	state raftpb.NODE_STATE

	term      uint64          // (etcd raft.raft.Term)
	votedFor  uint64          // (etcd raft.raft.Vote)
	votedFrom map[uint64]bool // (etcd raft.raft.votes)

	leaderID     uint64               // (etcd raft.raft.lead)
	idToProgress map[uint64]*Progress // (etcd raft.raft.prs)

	heartbeatTick        int // for leader
	heartbeatTickElapsed int // for leader

	logger      Logger
	raftStorage *raftStorage
	msgs        []raftpb.Message

	electionTick        int
	electionTickElapsed int

	// electionTickRandomized is the random number between
	// [electionTick, 2 * electionTick - 1], and gets reset
	// when raftNode state changes to follower or to candidate.
	electionTickRandomized int
	rand                   *rand.Rand

	tickFunc func()
	stepFunc func(r *raftNode, msg raftpb.Message)

	leaderCheckQuorum bool

	maxEntryPerMsg  uint64
	maxInflightMsgs int

	// pendingConfigExist is true, then new configuration will be ignored,
	// in preference to the unapplied configuration.
	pendingConfigExist bool

	// leaderIDTransferee is the ID of the leader transfer target
	// when it's not zero (Raft 3.10).
	leaderIDTransferee uint64
}
