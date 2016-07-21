package raft

import (
	"math/rand"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.stateMachine)
type stateMachine interface {
	Step(msg raftpb.Message) error
	readAndClearMailbox() []raftpb.Message
}

// (etcd raft.raftNode.readMessages)
func (rnd *raftNode) readAndClearMailbox() []raftpb.Message {
	msgs := rnd.mailbox
	rnd.mailbox = make([]raftpb.Message, 0)

	return msgs
}

type blackHole struct{}

func (blackHole) Step(raftpb.Message) error             { return nil }
func (blackHole) readAndClearMailbox() []raftpb.Message { return nil }

var noOpBlackHole = &blackHole{}

type connection struct {
	from, to uint64
}

// fakeNetwork simulates network message passing for Raft tests.
//
// (etcd raft.network)
type fakeNetwork struct {
	allStateMachines         map[uint64]stateMachine
	allStableStorageInMemory map[uint64]*StorageStableInMemory

	allDroppedConnections  map[connection]float64
	allIgnoredMessageTypes map[raftpb.MESSAGE_TYPE]bool
}

// (etcd raft.newNetwork)
func newFakeNetwork(machines ...stateMachine) *fakeNetwork {
	peerIDs := generateIDs(len(machines))

	allStateMachines := make(map[uint64]stateMachine, len(peerIDs))
	allStableStorageInMemory := make(map[uint64]*StorageStableInMemory, len(peerIDs))

	for i := range machines {
		id := peerIDs[i]
		switch v := machines[i].(type) {
		case nil:
			allStableStorageInMemory[id] = NewStorageStableInMemory()
			allStateMachines[id] = newTestRaftNode(id, peerIDs, 10, 1, allStableStorageInMemory[id])

		case *raftNode:
			v.id = id
			v.allProgresses = make(map[uint64]*Progress)
			for _, pid := range peerIDs {
				v.allProgresses[pid] = &Progress{}
			}
			v.resetWithTerm(0)
			allStateMachines[id] = v

		case *blackHole:
			allStateMachines[id] = v

		default:
			raftLogger.Panicf("unknown state machine type: %T", v)
		}
	}

	return &fakeNetwork{
		allStateMachines:         allStateMachines,
		allStableStorageInMemory: allStableStorageInMemory,

		allDroppedConnections:  make(map[connection]float64),
		allIgnoredMessageTypes: make(map[raftpb.MESSAGE_TYPE]bool),
	}
}

// (etcd raft.network.send)
func (fn *fakeNetwork) stepFirstFrontMessage(msgs ...raftpb.Message) {
	for len(msgs) > 0 {
		m := msgs[0]
		st := fn.allStateMachines[m.To]
		st.Step(m)

		msgs = append(msgs[1:], fn.filter(st.readAndClearMailbox())...)
	}
}

// (etcd raft.network.filter)
func (fn *fakeNetwork) filter(msgs []raftpb.Message) []raftpb.Message {
	var filtered []raftpb.Message
	for _, msg := range msgs {
		if fn.allIgnoredMessageTypes[msg.Type] {
			continue
		}

		switch msg.Type {
		case raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN:
			raftLogger.Panicf("%q never goes over network", msg.Type)

		default:
			percentage := fn.allDroppedConnections[connection{from: msg.From, to: msg.To}]
			if rand.Float64() < percentage {
				continue // skip append
			}
		}

		filtered = append(filtered, msg)
	}

	return filtered
}

// recoverAll recovers all dropped connections and resets ignored message types.
//
// (etcd raft.network.recover)
func (fn *fakeNetwork) recoverAll() {
	fn.allDroppedConnections = make(map[connection]float64)
	fn.allIgnoredMessageTypes = make(map[raftpb.MESSAGE_TYPE]bool)
}

// (etcd raft.network.drop)
func (fn *fakeNetwork) dropConnectionByPercentage(from, to uint64, percentage float64) {
	fn.allDroppedConnections[connection{from, to}] = percentage
}

// (etcd raft.network.cut)
func (fn *fakeNetwork) cutConnection(id1, id2 uint64) {
	fn.allDroppedConnections[connection{id1, id2}] = 1
	fn.allDroppedConnections[connection{id2, id1}] = 1
}

// isolate cuts all outgoing, incoming connections.
//
// (etcd raft.network.isolate)
func (fn *fakeNetwork) isolate(id uint64) {
	for sid := range fn.allStateMachines {
		if id != sid {
			fn.cutConnection(id, sid)
		}
	}
}

// (etcd raft.network.ignore)
func (fn *fakeNetwork) ignoreMessageType(tp raftpb.MESSAGE_TYPE) {
	fn.allIgnoredMessageTypes[tp] = true
}
