package raft

import (
	"context"

	"github.com/gyuho/db/raft/raftpb"
)

// Node defines the interface of a node in Raft cluster.
//
// (etcd raft.Node)
type Node interface {
	// GetNodeStatus returns the current status of the Raft state machine.
	//
	// (etcd raft.Node.Status)
	GetNodeStatus() NodeStatus

	// Tick increments the internal logical clock in the Node, by a single tick.
	// Election timeouts and heartbeat timeouts are in units of ticks.
	//
	// (etcd raft.Node.Tick)
	Tick()

	// Step advances the state machine based on the given raftpb.Message.
	//
	// (etcd raft.Node.Step)
	Step(ctx context.Context, msg raftpb.Message) error

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

	// Stop stops(terminates) the Node.
	//
	// (etcd raft.Node.Stop)
	Stop()

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

	// ReportUnreachable reports that Node with the given ID is not reachable for the last send.
	//
	// (etcd raft.Node.ReportUnreachable)
	ReportUnreachable(targetID uint64)

	// ReportSnapshot reports the status of sent snapshot.
	//
	// (etcd raft.Node.ReportSnapshot)
	ReportSnapshot(targetID uint64, status raftpb.SNAPSHOT_STATUS)
}

// node implements Node interface.
type node struct {
	tickCh chan struct{}

	proposeCh chan raftpb.Message
	receiveCh chan raftpb.Message

	configChangeCh chan raftpb.ConfigChange
	configStateCh  chan raftpb.ConfigState

	nodeReadyCh chan NodeReady
	advanceCh   chan struct{}

	stopCh chan struct{}
	doneCh chan struct{} // <-nd.stopCh ➝ close(doneCh)

	statusChCh chan chan NodeStatus
}

// tickChBufferSize buffers node.tickCh, so Raft node can buffer some ticks
// when the node is busy processing Raft messages. Raft node will resume
// processing buffered ticks when it becomes idle.
const tickChBufferSize = 128

// (etcd raft.newNode)
func newNode() node {
	return node{
		tickCh: make(chan struct{}, tickChBufferSize),

		proposeCh: make(chan raftpb.Message),
		receiveCh: make(chan raftpb.Message),

		configChangeCh: make(chan raftpb.ConfigChange),
		configStateCh:  make(chan raftpb.ConfigState),

		nodeReadyCh: make(chan NodeReady),
		advanceCh:   make(chan struct{}),

		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),

		statusChCh: make(chan chan NodeStatus),
	}
}

// (etcd raft.node.Status)
func (nd *node) GetNodeStatus() NodeStatus {
	ch := make(chan NodeStatus)
	nd.statusChCh <- ch
	return <-ch
}

// (etcd raft.node.Tick)
func (nd *node) Tick() {
	select {
	case nd.tickCh <- struct{}{}:

	case <-nd.doneCh:

	default:
		raftLogger.Warningln("Tick missed to fire, since Node was blocking too long!")
	}
}

// (etcd raft.node.step)
func (nd *node) step(ctx context.Context, msg raftpb.Message) error {
	chToReceive := nd.receiveCh
	if msg.Type == raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER {
		chToReceive = nd.proposeCh
	}

	select {
	case chToReceive <- msg:
		return nil

	case <-ctx.Done():
		return ctx.Err()

	case <-nd.doneCh:
		return ErrStopped
	}
}

// (etcd raft.node.Step)
func (nd *node) Step(ctx context.Context, msg raftpb.Message) error {
	if raftpb.IsInternalMessage(msg.Type) {
		// ignore unexpected local messages received over network
		raftLogger.Warningf("Step received internal message %q from network", msg.Type)
		return nil
	}
	return nd.step(ctx, msg)
}

// (etcd raft.node.Campaign)
func (nd *node) Campaign(ctx context.Context) error {
	return nd.step(ctx, raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_FOLLOWER_OR_CANDIDATE_TO_START_CAMPAIGN,
	})
}

// (etcd raft.node.Propose)
func (nd *node) Propose(ctx context.Context, data []byte) error {
	return nd.step(ctx, raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		Entries: []raftpb.Entry{{Data: data}},
	})
}

// (etcd raft.node.ProposeConfChange)
func (nd *node) ProposeConfigChange(ctx context.Context, cc raftpb.ConfigChange) error {
	data, err := cc.Marshal()
	if err != nil {
		return err
	}
	return nd.Step(ctx, raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER,
		Entries: []raftpb.Entry{{Type: raftpb.ENTRY_TYPE_CONFIG_CHANGE, Data: data}},
	})
}

// (etcd raft.node.ApplyConfChange)
func (nd *node) ApplyConfigChange(cc raftpb.ConfigChange) *raftpb.ConfigState {
	select {
	case nd.configChangeCh <- cc:
	case <-nd.doneCh:
	}

	var configState raftpb.ConfigState
	select {
	case configState = <-nd.configStateCh:
	case <-nd.doneCh:
	}

	return &configState
}

// (etcd raft.node.Stop)
func (nd *node) Stop() {
	select {
	case nd.stopCh <- struct{}{}:
		// not stopped yet, so trigger stop

	case <-nd.doneCh: // node has already been stopped, no need to do anything
		return
	}

	// wait until Stop has been acknowledged by node.run()
	<-nd.doneCh
}

// (etcd raft.node.Ready)
func (nd *node) NodeReady() <-chan NodeReady {
	return nd.nodeReadyCh
}

// (etcd raft.node.Advance)
func (nd *node) Advance() {
	select {
	case nd.advanceCh <- struct{}{}:
	case <-nd.doneCh:
	}
}

// (etcd raft.node.ReportUnreachable)
func (nd *node) ReportUnreachable(targetID uint64) {
	select {
	case nd.receiveCh <- raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_LEADER_CANNOT_CONNECT_TO_FOLLOWER,
		From: targetID}:

	case <-nd.doneCh:
	}
}

// (etcd raft.node.ReportSnapshot)
func (nd *node) ReportSnapshot(targetID uint64, status raftpb.SNAPSHOT_STATUS) {
	select {
	case nd.receiveCh <- raftpb.Message{
		Type:   raftpb.MESSAGE_TYPE_INTERNAL_RESPONSE_TO_SNAPSHOT_FROM_LEADER,
		From:   targetID,
		Reject: status == raftpb.SNAPSHOT_STATUS_FAILED}:

	case <-nd.doneCh:
	}
}

// (etcd raft.node.ReadIndex)
func (nd *node) RequestReadLeaderCurrentCommittedIndex(ctx context.Context, fromID uint64, data []byte) error {
	return nd.step(ctx, raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_READ_LEADER_CURRENT_COMMITTED_INDEX,
		From:    fromID,
		Entries: []raftpb.Entry{{Data: data}},
	})
}
