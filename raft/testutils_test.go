package raft

import "github.com/gyuho/db/raft/raftpb"

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

type connection struct {
	from, to uint64
}

// (etcd raft.network)
type fakeCluster struct {
	allStateMachines         map[uint64]stateMachine
	allStableStorageInMemory map[uint64]*StorageStableInMemory

	allDroppedConnection  map[connection]float64
	allIgnoredMessageType map[raftpb.MESSAGE_TYPE]bool
}

// (etcd raft.newNetwork)
func newFakeCluster(machines ...stateMachine) *fakeCluster {
	peerIDs := generateIDs(len(machines))

	allStateMachines := make(map[uint64]stateMachine)
	allStableStorageInMemory := make(map[uint64]*StorageStableInMemory)

	for i := range machines {
		id := peerIDs[i]
		switch v := machines[i].(type) {
		case nil:
			allStateMachines[id] = newRaftNode(&Config{
				ID:         id,
				allPeerIDs: peerIDs,

				ElectionTickNum:         10,
				HeartbeatTimeoutTickNum: 1,
				LeaderCheckQuorum:       false,
				StorageStable:           NewStorageStableInMemory(),
				MaxEntryNumPerMsg:       0,
				MaxInflightMsgNum:       256,
				LastAppliedIndex:        0,
			})
			allStableStorageInMemory[id] = NewStorageStableInMemory()

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

	return &fakeCluster{
		allStateMachines:         allStateMachines,
		allStableStorageInMemory: allStableStorageInMemory,

		allDroppedConnection:  make(map[connection]float64),
		allIgnoredMessageType: make(map[raftpb.MESSAGE_TYPE]bool),
	}
}

func generateIDs(n int) []uint64 {
	ids := make([]uint64, n)
	for i := 0; i < n; i++ {
		ids[i] = 1 + uint64(i)
	}
	return ids
}
