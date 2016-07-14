package raft

import "github.com/gyuho/db/raft/raftpb"

// Step defines how each Raft node behaves for the given message.
// State specific step function gets called at the end.
//
// (etcd raft.raft.Step)
func (rnd *raftNode) Step(msg raftpb.Message) error {
	if msg.Type == raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN { // m.Type == pb.MsgHup
		if rnd.state != raftpb.NODE_STATE_LEADER {
			raftLogger.Infof("%q %x is starting a new election(campaign) at term %d", rnd.state, rnd.id, rnd.term)
			rnd.followerBecomeCandidateAndStartCampaign()
		} else {
			raftLogger.Infof("%q %x ignores %q", rnd.state, rnd.id, msg.Type)
		}
		return nil
	}

	if msg.Type == raftpb.MESSAGE_TYPE_INTERNAL_TRANSFER_LEADER {
		if rnd.state != raftpb.NODE_STATE_LEADER {
			raftLogger.Infof("%q %x is ignoring %q to %x because it's not a leader", rnd.state, rnd.id, msg.Type, msg.From)
		}
	}

	switch {
	case msg.LogTerm == 0:
	// local message

	case msg.LogTerm > rnd.term:
		leaderID := msg.From
		if msg.Type == raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE {
			// ???
			// leader check quorum is true, not candidate, election hasn't timed out
			if rnd.leaderCheckQuorum && rnd.state != raftpb.NODE_STATE_CANDIDATE && rnd.electionTimeoutTickNum > rnd.electionTimeoutElapsedTickNum {
				raftLogger.Infof("%q %x [log index=%d | log term=%d | voted for %x] ignores vote from %x [log index=%d | log term=%d] at term %d (remaining election timeout ticks %d)",
					rnd.state, rnd.id, rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm(), rnd.votedFor, msg.From, msg.LogIndex, msg.LogTerm, rnd.term,
					rnd.electionTimeoutTickNum-rnd.electionTimeoutElapsedTickNum)
				return nil
			}
			leaderID = NoNodeID
		}
		raftLogger.Infof("%q %x [log term=%d] has received %q with higher term from %x [log term=%d], so reverting back to follower",
			rnd.state, rnd.id, rnd.term, msg.Type, msg.From, msg.LogTerm)
		rnd.becomeFollower(msg.LogTerm, leaderID)

	case msg.LogTerm < rnd.term:
		if rnd.leaderCheckQuorum && (msg.Type == raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT || msg.Type == raftpb.MESSAGE_TYPE_APPEND_FROM_LEADER) {
			// ???
			rnd.sendToMailbox(raftpb.Message{
				Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_APPEND_FROM_LEADER,
				To:   msg.From,
			})
		} else {
			raftLogger.Infof("%q %x [log term=%d] ignores %q with lower term from %x [log term=%d]", rnd.state, rnd.id, rnd.term, msg.Type, msg.From, msg.LogTerm)
		}
		return nil
	}

	rnd.stepFunc(rnd, msg)
	return nil
}
