package raft

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd raft.stateMachine)
type stateMachine interface {
	Step(msg raftpb.Message) error
	readResetMailbox() []raftpb.Message
}

func (rnd *raftNode) readResetMailbox() []raftpb.Message {
	msgs := rnd.mailbox
	rnd.mailbox = make([]raftpb.Message, 0)

	return msgs
}

type blackHole struct{}

func (blackHole) Step(raftpb.Message) error          { return nil }
func (blackHole) readResetMailbox() []raftpb.Message { return nil }

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

	allDroppedConnection  map[connection]float64
	allIgnoredMessageType map[raftpb.MESSAGE_TYPE]bool
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
			allStateMachines[id] = newRaftNode(&Config{
				ID:                      id,
				allPeerIDs:              peerIDs,
				ElectionTickNum:         10,
				HeartbeatTimeoutTickNum: 1,
				CheckQuorum:             false,
				StorageStable:           allStableStorageInMemory[id],
				MaxEntryNumPerMsg:       0,
				MaxInflightMsgNum:       256,
				LastAppliedIndex:        0,
			})

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

		allDroppedConnection:  make(map[connection]float64),
		allIgnoredMessageType: make(map[raftpb.MESSAGE_TYPE]bool),
	}
}

// (etcd raft.network.send)
func (fn *fakeNetwork) stepFirstFrontMessage(msgs ...raftpb.Message) {
	for len(msgs) > 0 {
		m := msgs[0]
		st := fn.allStateMachines[m.To]
		st.Step(m)

		msgs = append(msgs[1:], fn.filter(st.readResetMailbox())...)
	}
}

// (etcd raft.network.filter)
func (fn *fakeNetwork) filter(msgs []raftpb.Message) []raftpb.Message {
	var filtered []raftpb.Message
	for _, msg := range msgs {
		if fn.allIgnoredMessageType[msg.Type] {
			continue
		}

		switch msg.Type {
		case raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN:
			raftLogger.Panicf("%q never goes over network", msg.Type)

		default:
			percentage := fn.allDroppedConnection[connection{from: msg.From, to: msg.To}]
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
	fn.allDroppedConnection = make(map[connection]float64)
	fn.allIgnoredMessageType = make(map[raftpb.MESSAGE_TYPE]bool)
}

// (etcd raft.network.drop)
func (fn *fakeNetwork) dropConnectionByPercentage(from, to uint64, percentage float64) {
	fn.allDroppedConnection[connection{from, to}] = percentage
}

// (etcd raft.network.cut)
func (fn *fakeNetwork) cutConnection(id1, id2 uint64) {
	fn.allDroppedConnection[connection{id1, id2}] = 1
	fn.allDroppedConnection[connection{id2, id1}] = 1
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
	fn.allIgnoredMessageType[tp] = true
}

// (etcd raft.newTestRaft)
func newTestRaftNode(id uint64, allPeerIDs []uint64, electionTick, heartbeatTick int, stableStorage StorageStable) *raftNode {
	return newRaftNode(&Config{
		ID:                      id,
		allPeerIDs:              allPeerIDs,
		ElectionTickNum:         electionTick,
		HeartbeatTimeoutTickNum: heartbeatTick,
		CheckQuorum:             false,
		StorageStable:           stableStorage,
		MaxEntryNumPerMsg:       0,
		MaxInflightMsgNum:       256,
		LastAppliedIndex:        0,
	})
}

// (etcd raft.ents)
func newTestRaftNodeWithTerms(terms ...uint64) *raftNode {
	st := NewStorageStableInMemory()
	for i := range terms {
		st.Append(raftpb.Entry{Index: uint64(i + 1), Term: terms[i]})
	}

	rnd := newRaftNode(&Config{
		ID:                      1, // to be overwritten in 'newFakeNetwork'
		allPeerIDs:              nil,
		ElectionTickNum:         10,
		HeartbeatTimeoutTickNum: 1,
		CheckQuorum:             false,
		StorageStable:           st,
		MaxEntryNumPerMsg:       0,
		MaxInflightMsgNum:       256,
		LastAppliedIndex:        0,
	})
	rnd.resetWithTerm(0)

	return rnd
}

// (etcd raft.nextEnts)
func persistALlUnstableAndApplyNextEntries(rnd *raftNode, st *StorageStableInMemory) []raftpb.Entry {
	// append all unstable entries to stable
	st.Append(rnd.storageRaftLog.unstableEntries()...)

	// lastIndex gets the last index from unstable storage first.
	// If it's not available, try to get the last index in stable storage.
	//
	// lastTerm returns the term of raftLog's last log entry.
	//
	rnd.storageRaftLog.persistedEntriesAt(rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm())

	appliedEntries := rnd.storageRaftLog.nextEntriesToApply()
	rnd.storageRaftLog.appliedTo(rnd.storageRaftLog.committedIndex)

	return appliedEntries
}

// (etcd raft.idsBySize)
func generateIDs(n int) []uint64 {
	ids := make([]uint64, n)

	for i := 0; i < n; i++ {
		ids[i] = uint64(i) + 1
	}
	return ids
}

func Test_generateIDs(t *testing.T) {
	ids := generateIDs(10)
	var prevID uint64
	for i, id := range ids {
		if i == 0 {
			prevID = id
			fmt.Printf("generated %x\n", id)
			continue
		}
		fmt.Printf("generated %x\n", id)
		if id == prevID {
			t.Fatalf("#%d: expected %x != %x", i, prevID, id)
		}

		id = prevID
	}
}
