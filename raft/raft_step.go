package raft

import (
	"bytes"
	"math"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
)

// Step defines how each Raft node behaves for the given message.
// State specific step function gets called at the end.
//
// (etcd raft.raft.Step)
func (rnd *raftNode) Step(msg raftpb.Message) error {
	if msg.Type == raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN { // m.Type == pb.MsgHup
		if rnd.state == raftpb.NODE_STATE_LEADER {
			raftLogger.Infof("%s is already leader, so ignores %q from %x", rnd.describe(), msg.Type, msg.From)
			return nil
		}

		idx1 := rnd.storageRaftLog.appliedIndex + 1
		idx2 := rnd.storageRaftLog.committedIndex + 1
		ents, err := rnd.storageRaftLog.slice(idx1, idx2, math.MaxUint64)
		if err != nil {
			raftLogger.Panicf("unexpected error getting uncommitted entries (%v)", err)
		}
		if nconf := countPendingConfigChange(ents); nconf > 0 &&
			rnd.storageRaftLog.committedIndex > rnd.storageRaftLog.appliedIndex {
			raftLogger.Warningf("%s cannot campaign since there are still %d pending config changes", rnd.describe(), nconf)
			return nil
		}

		raftLogger.Infof("%s starts a new election", rnd.describe())
		rnd.becomeCandidateAndCampaign(raftpb.CAMPAIGN_TYPE_LEADER_ELECTION)
		return nil
	}

	switch {
	case msg.SenderCurrentTerm == 0: // local message (e.g. msg.Type == raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER)

	case msg.SenderCurrentTerm > rnd.currentTerm: // message with higher term
		leaderID := msg.From
		if msg.Type == raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE {
			// if the vote was requested for leadership transfer,
			// current node should not ignore this vote-request,
			// so that it can revert back to follower
			notLeaderTransfer := !bytes.Equal(msg.Context, []byte(raftpb.CAMPAIGN_TYPE_LEADER_TRANSFER.String()))

			// WHEN candidate receives a vote-request with higher term,
			// it SHOULD revert back to follower, no matter what.
			// (it SHOULD NOT IGNORE the vote-request)
			//
			// WHEN leader receives a vote-request with higher term,
			// it SHOULD IGNORE the vote-request
			// iff checkQuorum is true and election timeout hasn't passed,
			// because it guarantees that the leader is still in lease
			//
			// WHEN follower receives a vote-request with higher term,
			// it SHOULD IGNORE the vote-request
			// iff checkQuorum is true and election timeout hasn't passed,
			// because it has been in contact with leader for the last election timeout

			// Previously, the checkQuorum flag required an election timeout to
			// expire before a node could cast its first vote. This change permits
			// the node to cast a vote at any time when the leader is not known,
			// including immediately after startup.
			// https://github.com/coreos/etcd/commit/a7a867c1e6f1bac339d8118c508818492acd316d
			leaderExist := rnd.leaderID != NoNodeID

			// (Raft §4.2.3 Disruptive servers, p.42)
			//
			// "if a server receives a RequestVote-request within the minimum election timeout
			// of hearing from a current leader, it does not update its term or grant its vote."
			//
			// this helps avoid disruptions from servers with old configuration
			//
			// If checkQuorum is true, leader checks if quorum of cluster are active for every election timeout.
			// Leader sends internal check-quorum message to trigger quorum-check
			// for every election timeout (raftNode.tickFuncLeaderHeartbeatTimeout).
			// Now, if quorum is not active, leader reverts back to follower.
			//
			// So if checkQuorum is true and election timeout has not happened yet,
			// leader is guaranteed to have been in contact with quorum within
			// the last election timeout, as a valid leader.
			//
			// So it shouldn't increase its term.
			//
			// So it's ok to to reject vote-request.
			lastQuorumChecked := rnd.checkQuorum && rnd.electionTimeoutTickNum > rnd.electionTimeoutElapsedTickNum

			ignoreHigherTermVoteRequest := notLeaderTransfer && leaderExist && lastQuorumChecked

			if ignoreHigherTermVoteRequest {
				raftLogger.Infof("%s ignores vote-request from %s with higher term %d", rnd.describe(), types.ID(msg.From), msg.SenderCurrentTerm)
				return nil
			}
			leaderID = NoNodeID // leader should revert back to follower
		}
		raftLogger.Infof("%s received vote-request from %s with higher term %d", rnd.describe(), types.ID(msg.From), msg.SenderCurrentTerm)
		rnd.becomeFollower(msg.SenderCurrentTerm, leaderID)

	case msg.SenderCurrentTerm < rnd.currentTerm: // message with lower term
		// checkQuorum is true, and message from leader with lower term
		if rnd.checkQuorum &&
			(msg.Type == raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT ||
				msg.Type == raftpb.MESSAGE_TYPE_LEADER_APPEND) {

			// messages from leader with lower term is possible with network delay or network partition.
			//
			// If check quorum is not true, the leader will read heartbeat response with higher term,
			// and following 'Step' method call will revert the leader back to follower.
			//
			// If check quorum is true, we may not advance the term on MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE,
			// so we need to generate other message to advance the term.
			rnd.sendToMailbox(raftpb.Message{
				Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND,
				To:   msg.From, // to leader
			})
			// this will update msg.SenderCurrentTerm with rnd.currentTerm

		} else {
			raftLogger.Infof("%s ignores vote-request from %s with lower term %d", rnd.describe(), types.ID(msg.From), msg.SenderCurrentTerm)
		}
		return nil
	}

	rnd.stepFunc(rnd, msg)
	return nil
}
