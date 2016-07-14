package raft

import "github.com/gyuho/db/raft/raftpb"

// Step defines how each Raft node behaves for the given message.
// State specific step function gets called at the end.
//
// (etcd raft.raft.Step)
func (rnd *raftNode) Step(msg raftpb.Message) error {
	if msg.Type == raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN { // m.Type == pb.MsgHup
		if rnd.state != raftpb.NODE_STATE_LEADER {
			raftLogger.Infof("%q %x starts a new election at term %d", rnd.state, rnd.id, rnd.term)
			rnd.followerBecomeCandidateAndStartCampaign()
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
	case msg.SenderCurrentTerm == 0:
	// local message

	case msg.SenderCurrentTerm > rnd.term:
		leaderID := msg.From
		if msg.Type == raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE {
			// ???
			// leader check quorum is true, not candidate, election hasn't timed out
			if rnd.leaderCheckQuorum && rnd.state != raftpb.NODE_STATE_CANDIDATE && rnd.electionTimeoutTickNum > rnd.electionTimeoutElapsedTickNum {
				raftLogger.Infof("%q %x [log index=%d | log term=%d | voted for %x] ignores vote from %x [log index=%d | log term=%d] at term %d (remaining election timeout ticks %d)",
					rnd.state, rnd.id, rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm(), rnd.votedFor, msg.From, msg.LogIndex, msg.SenderCurrentTerm, rnd.term,
					rnd.electionTimeoutTickNum-rnd.electionTimeoutElapsedTickNum)
				return nil
			}
			leaderID = NoNodeID
		}

		raftLogger.Infof("%q %x [log term=%d] received %q from %x, with higher term [current log term=%d]",
			rnd.state, rnd.id, rnd.term, msg.Type, msg.From, msg.SenderCurrentTerm)
		rnd.becomeFollower(msg.SenderCurrentTerm, leaderID)

	case msg.SenderCurrentTerm < rnd.term:
		if rnd.leaderCheckQuorum && (msg.Type == raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT || msg.Type == raftpb.MESSAGE_TYPE_APPEND_FROM_LEADER) {
			// ???
			rnd.sendToMailbox(raftpb.Message{
				Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_APPEND_FROM_LEADER,
				To:   msg.From,
			})
		} else {
			raftLogger.Infof("%q %x [log term=%d] ignores %q with lower term from %x [current log term=%d]", rnd.state, rnd.id, rnd.term, msg.Type, msg.From, msg.SenderCurrentTerm)
		}
		return nil
	}

	rnd.stepFunc(rnd, msg)
	return nil
}
