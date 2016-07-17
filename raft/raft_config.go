package raft

import (
	"errors"
	"fmt"
)

// Config contains the parameters to start a Raft node.
type Config struct {
	// Logger implements system logging for Raft.
	Logger Logger

	// ID is the id of the Raft node, and 0 when there's no leader.
	//
	// (etcd raft.Config.ID)
	ID uint64

	// allPeerIDs contains the IDs of all peers and the node itself.
	// It should only be set when starting a new Raft cluster.
	//
	// (etcd raft.Config.peers)
	allPeerIDs []uint64

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

	// LeaderCheckQuorum is true, then a leader checks if quorum is active,
	// for an election timeout. If not, the leader steps down.
	LeaderCheckQuorum bool

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
