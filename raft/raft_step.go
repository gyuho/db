package raft

import (
	"bytes"

	"github.com/gyuho/db/raft/raftpb"
)

// Step defines how each Raft node behaves for the given message.
// State specific step function gets called at the end.
//
// (etcd raft.raft.Step)
func (rnd *raftNode) Step(msg raftpb.Message) error {
	if msg.Type == raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN { // m.Type == pb.MsgHup
		if rnd.state != raftpb.NODE_STATE_LEADER {
			raftLogger.Infof("%q %x starts a new election at term %d", rnd.state, rnd.id, rnd.term)
			rnd.followerBecomeCandidateAndStartCampaign(raftpb.CAMPAIGN_TYPE_LEADER_ELECTION)
		} else {
			raftLogger.Infof("%q %x ignores %q", rnd.state, rnd.id, msg.Type)
		}
		return nil
	}

	if msg.Type == raftpb.MESSAGE_TYPE_INTERNAL_TRANSFER_LEADER {
		if rnd.state != raftpb.NODE_STATE_LEADER {
			raftLogger.Infof("%q %x ignores %q to %x because it's not a leader", rnd.state, rnd.id, msg.Type, msg.From)
		}
	}

	switch {
	case msg.SenderCurrentTerm == 0: // local message (e.g. msg.Type == raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER)

	case msg.SenderCurrentTerm > rnd.term: // message with higher term

		leaderID := msg.From
		if msg.Type == raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE {
			// if the vote was requested for leadership transfer
			// this node should not ignore this vote request
			// and reverts back to follower if needed
			isLeaderTransfer := bytes.Equal(msg.Context, []byte(raftpb.CAMPAIGN_TYPE_LEADER_TRANSFER.String()))

			// (Raft §4.2.3 Disruptive servers, p.42)
			// 1. state must not be candidate, for this node to be ok to reject vote request
			//
			// 2. leaderCheckQuorum is true
			// AND
			// 3. election time out hasn't passed yet
			//
			// THEN leader last confirmed its leadership, guaranteed to have been
			// in contact with quorum within the election timeout, so it shouldn't
			// increase its term either.
			//
			// SO, it's ok to to reject vote request
			//
			// "if a server receives a RequestVote request within the minimum election timeout
			// of hearing from a current leader, it does not update its term or grant its vote."
			//
			// this helps avoid disruptions from servers with old configuration
			//
			quorumLease := rnd.state != raftpb.NODE_STATE_CANDIDATE &&
				rnd.leaderCheckQuorum &&
				rnd.electionTimeoutTickNum > rnd.electionTimeoutElapsedTickNum

			okToIgnore := !isLeaderTransfer && quorumLease

			if okToIgnore {
				raftLogger.Infof(`

	%q %x [log index=%d | log term=%d | voted for %x]
	ignores %q
	from %x [log index=%d | log term=%d] at term %d
	(remaining election timeout ticks %d)

`,
					rnd.state, rnd.id, rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm(), rnd.votedFor,
					msg.Type, msg.From, msg.LogIndex, msg.SenderCurrentTerm, rnd.term,
					rnd.electionTimeoutTickNum-rnd.electionTimeoutElapsedTickNum)

				return nil
			}
			leaderID = NoNodeID
		}

		raftLogger.Infof("%q %x [log term=%d] received %q from %x, with higher term [current log term=%d]",
			rnd.state, rnd.id, rnd.term, msg.Type, msg.From, msg.SenderCurrentTerm)
		rnd.becomeFollower(msg.SenderCurrentTerm, leaderID)

	case msg.SenderCurrentTerm < rnd.term: // message with lower term

		// checkQuorum is true, and message from leader with lower term
		if rnd.leaderCheckQuorum &&
			(msg.Type == raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT ||
				msg.Type == raftpb.MESSAGE_TYPE_APPEND_FROM_LEADER) {

			// messages from leader with lower term is possible with network delay or network partition.
			//
			// If check quorum is not true, the leader will read heartbeat response with higher term,
			// and reverts back to follower.
			//
			// If check quorum is true, we may not advance the term on MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE,
			// so we need to generate other message to advance the term.
			rnd.sendToMailbox(raftpb.Message{
				Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_APPEND_FROM_LEADER,
				To:   msg.From, // to leader
			})
			// this will update msg.SenderCurrentTerm with rnd.term
			// and the leader will match with the previous 'case'
			// and reverts back to follower

		} else {
			raftLogger.Infof("%q %x [log term=%d] ignores %q with lower term from %x [current log term=%d]", rnd.state, rnd.id, rnd.term, msg.Type, msg.From, msg.SenderCurrentTerm)
		}
		return nil
	}

	rnd.stepFunc(rnd, msg)
	return nil
}
