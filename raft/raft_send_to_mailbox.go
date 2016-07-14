package raft

import "github.com/gyuho/db/raft/raftpb"

// sendToMailbox sends a message, given that the requested message
// has already set msg.To for its receiver.
//
// (etcd raft.raft.send)
func (rnd *raftNode) sendToMailbox(msg raftpb.Message) {
	msg.From = rnd.id

	// proposal must go through consensus, which means
	// proposal is to be forwarded to the leader,
	// and replicated back to followers.
	// so it should be treated as local message
	// by setting msg.LogTerm as 0
	if msg.Type != raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER {
		msg.LogTerm = rnd.term
	}

	rnd.mailbox = append(rnd.mailbox, msg)
}
