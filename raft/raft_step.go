package raft

import "github.com/gyuho/db/raft/raftpb"

// Step defines how each Raft node behaves for the given message.
// State specific step function gets called at the end.
//
// (etcd raft.raft.Step)
func (rnd *raftNode) Step(msg raftpb.Message) error {

	return nil
}
