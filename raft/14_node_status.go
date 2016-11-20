package raft

import (
	"fmt"
	"strings"

	"github.com/gyuho/db/raft/raftpb"
)

// Status represents the status of raft node.
//
// (etcd raft.Status)
type Status struct {
	ID uint64

	SoftState raftpb.SoftState
	HardState raftpb.HardState

	AppliedIndex uint64

	AllProgresses map[uint64]Progress
}

// (etcd raft.getStatus)
func getStatus(rnd *raftNode) Status {
	ns := Status{
		ID: rnd.id,

		SoftState: *rnd.softState(),
		HardState: rnd.hardState(),

		AppliedIndex: rnd.storageRaftLog.appliedIndex,
	}

	if ns.SoftState.NodeState == raftpb.NODE_STATE_LEADER {
		idToProgress := make(map[uint64]Progress)
		for id, prog := range rnd.allProgresses {
			idToProgress[id] = *prog
		}
		ns.AllProgresses = idToProgress
	}

	return ns
}

// MarshalJSON marshals Status to bytes.
//
// (etcd raft.Status.MarshalJSON)
func (ns Status) MarshalJSON() ([]byte, error) {
	txt := fmt.Sprintf(`{"id":"%x","voted_for":"%x","committed_index":%d,"term":%d,"leader_id":"%x","node_state":%q,"progress":{`,
		ns.ID, ns.HardState.VotedFor, ns.HardState.CommittedIndex, ns.HardState.Term, ns.SoftState.LeaderID, ns.SoftState.NodeState)
	if len(ns.AllProgresses) > 0 {
		txts := make([]string, 0, len(ns.AllProgresses))
		for id, prog := range ns.AllProgresses {
			txts = append(txts, fmt.Sprintf(`"%x":{"match_index":%d,"next_index":%d,"node_state":%q}`, id, prog.MatchIndex, prog.NextIndex, prog.State))
		}
		txt += strings.Join(txts, ",")
	}
	txt += "}}"
	return []byte(txt), nil
}

func (ns Status) String() string {
	b, err := ns.MarshalJSON()
	if err != nil {
		raftLogger.Panicf("unexpected error %v", err)
	}
	return string(b)
}
