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

	// Ready returns a channel that receives point-in-time state of Node.
	// 'Advance' method MUST be followed, AFTER APPLYING the state in Ready.
	//
	// (etcd raft.Node.Ready)
	Ready() <-chan Ready

	// Advance notifies the Node that the application has saved the progress
	// up to the last Ready state. And it prepares the Node to return the
	// next point-in-time state, Ready.
	//
	// The application MUST call 'Advance' AFTER it applies the entries in the
	// last Ready state.
	//
	// However, as an optimization, the application may call Advance
	// WHILE it is applying the commands.
	//
	// For example, when the last Ready contains a snapshot, the application
	// might take a long time to apply the snapshot data. To continue receiving
	// Ready without blocking Raft progress, it can call Advance before
	// finishing applying the last Ready.
	//
	// When an application receives Ready where SoftState.NodeState is Candidate,
	// it must apply all pending configuration changes if any.
	//
	//   nr := <-nd.Ready()
	//   go apply(nr.EntriesToCommit)
	//   if nr.SoftState.NodeState == Candidate { waitAllApplied() }
	//   nd.Advance()
	//
	// (etcd raft.Node.Advance)
	Advance()

	// ReadIndex requests a read state, set in the Ready.
	//
	// (etcd raft.Node.ReadIndex)
	ReadIndex(ctx context.Context, rctx []byte) error

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
	tickCh chan struct{} // tickc chan struct{}

	incomingProposalMessageCh chan raftpb.Message // propc chan pb.Message
	incomingMessageCh         chan raftpb.Message // recvc chan pb.Message

	configChangeCh chan raftpb.ConfigChange // confc chan pb.ConfChange
	configStateCh  chan raftpb.ConfigState  // confstatec chan pb.ConfState

	readyCh   chan Ready    // readyc chan Ready
	advanceCh chan struct{} // advancec chan struct{}

	stopCh chan struct{} // stop chan struct{}
	doneCh chan struct{} // done  chan struct{}
	// <-nd.stopCh ➝ close(doneCh)

	nodeStatusChCh chan chan NodeStatus // status chan chan Status
}

// tickChBufferSize buffers node.tickCh, so Raft node can buffer some ticks
// when the node is busy processing Raft messages. Raft node will resume
// processing buffered ticks when it becomes idle.
const tickChBufferSize = 128

// (etcd raft.newNode)
func newNode() node {
	return node{
		tickCh: make(chan struct{}, tickChBufferSize),

		incomingProposalMessageCh: make(chan raftpb.Message),
		incomingMessageCh:         make(chan raftpb.Message),

		configChangeCh: make(chan raftpb.ConfigChange),
		configStateCh:  make(chan raftpb.ConfigState),

		readyCh:   make(chan Ready),
		advanceCh: make(chan struct{}),

		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),

		nodeStatusChCh: make(chan chan NodeStatus),
	}
}

// (etcd raft.node.Status)
func (nd *node) GetNodeStatus() NodeStatus {
	ch := make(chan NodeStatus)
	nd.nodeStatusChCh <- ch
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
	chToReceive := nd.incomingMessageCh
	if msg.Type == raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER {
		chToReceive = nd.incomingProposalMessageCh
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
		raftLogger.Warningf("node.Step got %q from network (so ignores)", msg.Type)
		return nil
	}
	return nd.step(ctx, msg)
}

// (etcd raft.node.Campaign)
func (nd *node) Campaign(ctx context.Context) error {
	return nd.step(ctx, raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
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
func (nd *node) Ready() <-chan Ready {
	return nd.readyCh
}

// (etcd raft.node.Advance)
func (nd *node) Advance() {
	select {
	case nd.advanceCh <- struct{}{}:
	case <-nd.doneCh:
	}
}

// ReadState provides the state of read-only query.
// The application must send raftpb.MESSAGE_TYPE_READ_INDEX
// first, before it reads ReadState from NodeReady.
//
// READ_INDEX is used to serve clients' read-only queries without
// going through Raft, but still with 'quorum-get' on. It bypasses the Raft log, but
// still preserves the linearizability of reads, with lower costs.
//
// If a request goes through Raft log, it needs replication, which requires synchronous
// disk writes in order to append those request entries to its log. Since read-only requests
// do not change any state of replicated state machine, these writes can be time- and
// resource-consuming.
//
// (Raft §6.4 Processing read-only queries more efficiently, p.72)
// To bypass the Raft log with linearizable reads:
//
//   1. If Leader has not yet committed an entry from SenderCurrentTerm, it waits until it has done so.
//
//   2. Leader saves its SenderCurrentCommittedIndex in a local variable 'readIndex', which is used
//      as a lower bound for the version of the state that read-only queries operate against.
//
//   3. Leader must ensure that it hasn't been superseded by a newer Leader,
//      by issuing a new round of heartbeats and waiting for responses from cluster quorum.
//
//   4. These responses from Followers acknowledging the Leader indicates that
//      there was no other Leader at the moment Leader sent out heartbeats.
//
//   5. Therefore, Leader's 'readIndex' was, at the time, the largest committed index,
//      ever seen by any node in the cluster.
//
//   6. Leader now waits for its state machine to advance at least as far as the 'readIndex'.
//      And this is current enought to satisfy linearizability.
//
//   7. Leader can now respond to those read-only client requests.
//
// (etcd raft.ReadState)
type ReadState struct {
	Index      uint64
	RequestCtx []byte
}

// (etcd raft.node.ReadIndex)
func (nd *node) ReadIndex(ctx context.Context, rctx []byte) error {
	return nd.step(ctx, raftpb.Message{
		Type:    raftpb.MESSAGE_TYPE_READ_INDEX,
		Entries: []raftpb.Entry{{Data: rctx}},
	})
}

// (etcd raft.node.ReportUnreachable)
func (nd *node) ReportUnreachable(targetID uint64) {
	select {
	case nd.incomingMessageCh <- raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_INTERNAL_LEADER_CANNOT_CONNECT_TO_FOLLOWER,
		From: targetID,
	}:

	case <-nd.doneCh:
	}
}

// (etcd raft.node.ReportSnapshot)
func (nd *node) ReportSnapshot(targetID uint64, status raftpb.SNAPSHOT_STATUS) {
	select {
	case nd.incomingMessageCh <- raftpb.Message{
		Type:   raftpb.MESSAGE_TYPE_INTERNAL_RESPONSE_TO_LEADER_SNAPSHOT,
		From:   targetID,
		Reject: status == raftpb.SNAPSHOT_STATUS_FAILED,
	}:

	case <-nd.doneCh:
	}
}

// (etcd raft.node.run)
func (nd *node) runWithRaftNode(rnd *raftNode) {
	var (
		leaderID = NoNodeID

		prevSoftState = rnd.softState()
		prevHardState = raftpb.EmptyHardState

		incomingProposalMessageCh chan raftpb.Message

		advanceCh chan struct{}

		rd      Ready
		readyCh chan Ready

		hasPrevLastUnstableIndex bool
		prevLastUnstableIndex    uint64

		prevLastUnstableTerm uint64

		prevSnapshotIndex uint64
	)

	for {
		// Advance notifies the Node that the application has saved the progress
		// up to the last Ready state. And it prepares the Node to return the
		// next point-in-time state, Ready.
		if advanceCh != nil {
			readyCh = nil
		} else {
			rd = newReady(rnd, prevSoftState, prevHardState)
			if rd.ContainsUpdates() {
				readyCh = nd.readyCh
			} else {
				readyCh = nil
			}
		}

		if rnd.leaderID != leaderID {
			if rnd.hasLeader() { // rnd.leaderID != NoNodeID
				if leaderID == NoNodeID {
					raftLogger.Infof("%s elected leader %x", rnd.describe(), rnd.leaderID)
				} else {
					raftLogger.Infof("%s changed its leader from %x to %x", rnd.describe(), leaderID, rnd.leaderID)
				}
				incomingProposalMessageCh = nd.incomingProposalMessageCh
			} else {
				raftLogger.Infof("%s lost leader %x", rnd.describe(), leaderID)
				incomingProposalMessageCh = nil
			}
			leaderID = rnd.leaderID
		}

		select {
		case <-nd.tickCh: // case <-n.tickc:
			rnd.tickFunc()

		case msg := <-incomingProposalMessageCh: // case m := <-propc:
			msg.From = rnd.id
			rnd.Step(msg)

		case msg := <-nd.incomingMessageCh: // case m := <-n.recvc:
			if _, ok := rnd.allProgresses[msg.From]; ok || !raftpb.IsResponseMessage(msg.Type) { // ???
				rnd.Step(msg)
			}

		case readyCh <- rd: // case readyc <- rd:
			// Ready returns a channel that receives point-in-time state of Node.
			// Advance() method must be followed, after applying the state in Ready.

			if rd.SoftState != nil {
				prevSoftState = rd.SoftState
			}

			if len(rd.EntriesToSave) > 0 {
				hasPrevLastUnstableIndex = true
				prevLastUnstableIndex = rd.EntriesToSave[len(rd.EntriesToSave)-1].Index
				prevLastUnstableTerm = rd.EntriesToSave[len(rd.EntriesToSave)-1].Term
			}

			if !raftpb.IsEmptyHardState(rd.HardStateToSave) {
				prevHardState = rd.HardStateToSave
			}

			if !raftpb.IsEmptySnapshot(rd.SnapshotToSave) {
				prevSnapshotIndex = rd.SnapshotToSave.Metadata.Index
			}

			rnd.mailbox = nil
			rnd.readState.Index = uint64(0)
			rnd.readState.RequestCtx = nil
			advanceCh = nd.advanceCh

		case <-advanceCh: // case <-advancec:
			if prevHardState.CommittedIndex != 0 {
				rnd.storageRaftLog.appliedTo(prevHardState.CommittedIndex)
			}

			if hasPrevLastUnstableIndex {
				rnd.storageRaftLog.persistedEntriesAt(prevLastUnstableIndex, prevLastUnstableTerm)
				hasPrevLastUnstableIndex = false // reset
			}

			rnd.storageRaftLog.persistedSnapshotAt(prevSnapshotIndex)
			advanceCh = nil // reset, waits for next ready

		case nodeStatusCh := <-nd.nodeStatusChCh: // case c := <-n.status:
			nodeStatusCh <- getNodeStatus(rnd)

		case <-nd.stopCh: // case <-n.stop:
			close(nd.doneCh)
			return

		case configChange := <-nd.configChangeCh: // case cc := <-n.confc:
			if configChange.NodeID == NoNodeID {
				rnd.resetPendingConfigExist()
				select {
				case nd.configStateCh <- raftpb.ConfigState{IDs: rnd.allNodeIDs()}:
				case <-nd.doneCh:
				}
				break
			}

			switch configChange.Type {
			case raftpb.CONFIG_CHANGE_TYPE_ADD_NODE:
				rnd.addNode(configChange.NodeID)

			case raftpb.CONFIG_CHANGE_TYPE_REMOVE_NODE:
				if configChange.NodeID == rnd.id {
					// block incominb proposal when local node is to be removed
					incomingProposalMessageCh = nil
				}
				rnd.deleteNode(configChange.NodeID)

			case raftpb.CONFIG_CHANGE_TYPE_UPDATE_NODE:
				rnd.resetPendingConfigExist()

			default:
				raftLogger.Panicf("%s has received unknown config change type %q", rnd.describe(), configChange.Type)
			}

			select {
			case nd.configStateCh <- raftpb.ConfigState{IDs: rnd.allNodeIDs()}:
			case <-nd.doneCh:
			}
		}
	}
}

// Peer contains peer ID and context data.
//
// (etcd raft.Peer)
type Peer struct {
	ID      uint64
	Context []byte
}

// StartNode returns a new Node with given configuration.
// It appends raftpb.CONFIG_CHANGE_TYPE_ADD_NODE to its initial log.
//
// (etcd raft.StartNode)
func StartNode(config *Config, peers []Peer) Node {
	rnd := newRaftNode(config)

	// start with term 1, no leader
	rnd.becomeFollower(1, NoNodeID)

	for _, peer := range peers {
		configChange := raftpb.ConfigChange{
			Type:    raftpb.CONFIG_CHANGE_TYPE_ADD_NODE,
			NodeID:  peer.ID,
			Context: peer.Context,
		}
		configChangeData, err := configChange.Marshal()
		if err != nil {
			raftLogger.Panicf("StartNode configChange.Marshal (%v)", err)
		}
		entry := raftpb.Entry{
			Type:  raftpb.ENTRY_TYPE_CONFIG_CHANGE,
			Index: rnd.storageRaftLog.lastIndex() + 1,
			Term:  1,
			Data:  configChangeData,
		}
		rnd.storageRaftLog.appendToStorageUnstable(entry)
	}

	// mark these initial entries as committed
	// (still unstable)
	rnd.storageRaftLog.commitTo(rnd.storageRaftLog.lastIndex())

	// now apply them, so that application can call Campaign right afterwards
	for _, peer := range peers {
		rnd.addNode(peer.ID)
	}

	nd := newNode()
	go nd.runWithRaftNode(rnd)
	return &nd
}
