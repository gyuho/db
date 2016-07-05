package raft

import (
	"context"

	"github.com/gyuho/db/raft/raftpb"
)

// Node defines the interface of a node in Raft cluster.
//
// (etcd raft.Node)
type Node interface {
	// GetStatus returns the current status of the Raft state machine.
	//
	// (etcd raft.Node.Status)
	GetStatus() Status

	// Tick increments the internal logical clock in the Node, by a single tick.
	// Election timeouts and heartbeat timeouts are in units of ticks.
	//
	// (etcd raft.Node.Tick)
	Tick()

	// Campaign changes the node state to Candidate, and starts a campaign to become Leader.
	//
	// (etcd raft.Node.Campaign)
	Campaign(ctx context.Context) error

	// Propose proposes data to be appended to Raft log.
	//
	// (etcd raft.Node.Propose)
	Propose(ctx context.Context, data []byte) error

	// ProposeConfigChange proposes configuration change.
	// At most one configuration change can be in process of Raft consensus.
	//
	// (etcd raft.Node.ProposeConfChange)
	ProposeConfigChange(ctx context.Context, cc raftpb.ConfigChange) error

	// ApplyConfigChange applies the configuration change to the local Node.
	// And returns raftpb.ConfigState.
	//
	// (etcd raft.Node.ApplyConfChange)
	ApplyConfigChange(cc raftpb.ConfigChange) *raftpb.ConfigState

	// Step advances the state machine based on the given raftpb.Message.
	//
	// (etcd raft.Node.Step)
	Step(ctx context.Context, msg raftpb.Message) error

	// NodeReady returns a channel that receives point-in-time state of Node.
	// Advance() method must be followed, after applying the state in NodeReady.
	//
	// (etcd raft.Node.NodeReady)
	NodeReady() <-chan NodeReady

	// Advance notifies the Node that the application has saved the progress
	// up to the last NodeReady state. And it prepares the Node to return the
	// next point-in-time state, NodeReady.
	//
	// The application should call Advance AFTER it applies the entries in the
	// last NodeReady state.
	//
	// However, as an optimization, the application may call Advance
	// WHILE it is applying the commands.
	//
	// For example, when the last NodeReady contains a snapshot, the application
	// might take a long time to apply the snapshot data. To continue receiving
	// NodeReady without blocking Raft progress, it can call Advance before
	// finishing applying the last NodeReady.
	//
	// When an application receives NodeReady where SoftState.NodeState is Candidate,
	// it must apply all pending configuration changes if any.
	//
	//   nr := <-nd.NodeReady()
	//   go apply(nr.EntriesToCommit)
	//   if nr.SoftState.NodeState == Candidate { waitAllApplied() }
	//   nd.Advance()
	//
	// (etcd raft.Node.Advance)
	Advance()

	// Stop stops(terminates) the Node.
	//
	// (etcd raft.Node.Stop)
	Stop()

	// ReportUnreachable reports that Node with the given ID is not reachable for the last send.
	//
	// (etcd raft.Node.ReportUnreachable)
	ReportUnreachable(targetID uint64)

	// ReportSnapshot reports the status of sent snapshot.
	//
	// (etcd raft.Node.ReportSnapshot)
	ReportSnapshot(targetID uint64, status raftpb.SNAPSHOT_STATUS)
}

// Peer contains peer ID and context data.
//
// (etcd raft.Peer)
type Peer struct {
	ID   uint64
	Data []byte
}
