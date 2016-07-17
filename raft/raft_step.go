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
			raftLogger.Infof("%s starts a new election", rnd.describe())
			rnd.becomeCandidateAndCampaign(raftpb.CAMPAIGN_TYPE_LEADER_ELECTION)
		} else {
			raftLogger.Infof("%s ignores %q from %x", rnd.describe(), msg.Type, msg.From)
		}
		return nil
	}

	if msg.Type == raftpb.MESSAGE_TYPE_INTERNAL_TRANSFER_LEADER {
		if rnd.state != raftpb.NODE_STATE_LEADER {
			raftLogger.Infof("%s ignores %q to %x because it's not a leader", rnd.describe(), msg.Type, msg.From)
		}
	}

	switch {
	case msg.SenderCurrentTerm == 0: // local message (e.g. msg.Type == raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER)

	case msg.SenderCurrentTerm > rnd.term: // message with higher term

		leaderID := msg.From
		if msg.Type == raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE {
			// if the vote was requested for leadership transfer
			// this node should not ignore this vote-request so that
			// it can revert back to follower
			isLeaderTransfer := bytes.Equal(msg.Context, []byte(raftpb.CAMPAIGN_TYPE_LEADER_TRANSFER.String()))

			// WHEN candidate receives a vote-request with higher term,
			// it SHOULD NOT IGNORE the vote-request and SHOULD revert back to follower
			//
			// WHEN leader receives a vote-request with higher term,
			// and checkQuorum is true and election timeout hasn't passed,
			// it SHOULD IGNORE the vote-request
			// because it guarantees that the leader is still in lease
			//
			// WHEN follower receives a vote-request with higher term,
			// and checkQuorum is true and election timeout hasn't passed,
			// it SHOULD IGNORE the vote-request
			// because it has been in contact with leader for the last election timeout
			//
			notCandidate := rnd.state != raftpb.NODE_STATE_CANDIDATE

			// (Raft §4.2.3 Disruptive servers, p.42)
			//
			// "if a server receives a RequestVote-request within the minimum election timeout
			// of hearing from a current leader, it does not update its term or grant its vote."
			//
			// this helps avoid disruptions from servers with old configuration
			//
			// If checkQuorum is true, leader checks if quorum of cluster are active for every election timeout
			// (if rnd.allProgresses[id].RecentActive {activeN++}).
			// And leader maintains 'Progress.RecentActive' for every incoming message from follower.
			// Now, if quorum is not active, leader reverts back to follower.
			//
			// Leader sends internal check-quorum message to trigger quorum-check
			// for every election timeout (raftNode.tickFuncLeaderHeartbeatTimeout).
			//
			// So if checkQuorum is true and election timeout has not happened yet,
			// then leader is guaranteed to have been in contact with quorum within
			// the last election timeout, as a valid leader.
			// So it shouldn't increase its term.
			//
			// SO, it's ok to to reject vote-request
			lastQuorumChecked := rnd.checkQuorum && rnd.electionTimeoutTickNum > rnd.electionTimeoutElapsedTickNum

			ignoreHigherTermVoteRequest := !isLeaderTransfer && notCandidate && lastQuorumChecked

			if ignoreHigherTermVoteRequest {
				raftLogger.Infof(`

	%s
	IGNORES %s
	which has HIGHER term (sender current log term '%d' > node current term '%d')
	(elapsed election timeout ticks: %d out of %d)
	(IGNORES VOTE-REQUEST from CANDIDATE with "HIGHER" term!)

`, rnd.describeLong(), raftpb.DescribeMessageLong(msg),
					msg.SenderCurrentTerm, rnd.term,
					rnd.electionTimeoutElapsedTickNum, rnd.electionTimeoutTickNum)

				return nil
			}
			leaderID = NoNodeID
		}

		raftLogger.Infof(`

	%s
	RECEIVED %s
	which has HIGHER term (sender current log term '%d' > node current term '%d')
	(elapsed election timeout ticks: %d out of %d)
	(GOT VOTE-REQUEST from CANDIDATE with HIGHER term, so need to BECOME FOLLOWER with sender term '%d')

`, rnd.describeLong(), raftpb.DescribeMessageLong(msg),
			msg.SenderCurrentTerm, rnd.term,
			rnd.electionTimeoutElapsedTickNum, rnd.electionTimeoutTickNum, msg.SenderCurrentTerm)

		rnd.becomeFollower(msg.SenderCurrentTerm, leaderID)

	case msg.SenderCurrentTerm < rnd.term: // message with lower term

		// checkQuorum is true, and message from leader with lower term
		if rnd.checkQuorum &&
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

			raftLogger.Infof(`
				
	%s
	IGNORES %s
	whic has LOWER term (sender current log term '%d' < node current term '%d')
	(IGNORES VOTE-REQUEST from candidate with LOWER term!)

`, rnd.describeLong(), raftpb.DescribeMessageLong(msg), msg.SenderCurrentTerm, rnd.term)

		}
		return nil
	}

	rnd.stepFunc(rnd, msg)

	return nil
}
