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
	// Handle the message term, which may result in our stepping down to a follower.
	switch {
	case msg.SenderCurrentTerm == 0:
		// local message (e.g. msg.Type == raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER)

	case msg.SenderCurrentTerm > rnd.currentTerm: // message with higher term
		// (Raft §3.4 Leader election, p.17)
		//
		// Raft randomizes election timeouts to minimize split votes or two candidates.
		// And each server votes for candidate on first-come base.
		// This randomized retry approach elects a leader rapidly, more obvious and understandable.
		//
		// leader or candidate rejects vote-requests from another server.

		leaderID := msg.From
		if msg.Type == raftpb.MESSAGE_TYPE_PRE_CANDIDATE_REQUEST_VOTE || msg.Type == raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE {
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
				raftLogger.Infof("%s ignores vote-request %s from %s with higher term %d", rnd.describe(), msg.Type, types.ID(msg.From), msg.SenderCurrentTerm)
				return nil
			}
			leaderID = NoNodeID // leader should revert back to follower
		}

		switch {
		case msg.Type == raftpb.MESSAGE_TYPE_PRE_CANDIDATE_REQUEST_VOTE:
			// Never change our term in response to a PreVote

		case msg.Type == raftpb.MESSAGE_TYPE_RESPONSE_TO_PRE_CANDIDATE_REQUEST_VOTE && !msg.Reject:
			// We send pre-vote requests with a term in our future.
			// If the pre-vote is granted, we will increment our term when we get a quorum.
			// If it is not, the term comes from the node that rejected our vote
			// so we should become a follower at the new term.

		default:
			raftLogger.Infof("%s received vote-request %s from %s with higher term %d", rnd.describe(), msg.Type, types.ID(msg.From), msg.SenderCurrentTerm)
			rnd.becomeFollower(msg.SenderCurrentTerm, leaderID)
		}

	case msg.SenderCurrentTerm < rnd.currentTerm: // message with lower term
		// We have received messages from a leader at a lower term. It is possible
		// that these messages were simply delayed in the network, but this could
		// also mean that this node has advanced its term number during a network
		// partition, and it is now unable to either win an election or to rejoin
		// the majority on the old term. If checkQuorum is false, this will be
		// handled by incrementing term numbers in response to MsgVote with a
		// higher term, but if checkQuorum is true we may not advance the term on
		// MsgVote and must generate other messages to advance the term. The net
		// result of these two features is to minimize the disruption caused by
		// nodes that have been removed from the cluster's configuration: a
		// removed node will send MsgVotes (or MsgPreVotes) which will be ignored,
		// but it will not receive MsgApp or MsgHeartbeat, so it will not create
		// disruptive term increases
		//
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

	switch msg.Type {
	case raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN: // pb.MsgHup
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
		if rnd.preVote {
			rnd.doCampaign(raftpb.CAMPAIGN_TYPE_PRE_LEADER_ELECTION)
		} else {
			rnd.doCampaign(raftpb.CAMPAIGN_TYPE_LEADER_ELECTION)
		}

	case raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE, raftpb.MESSAGE_TYPE_PRE_CANDIDATE_REQUEST_VOTE:
		// The msg.SenderCurrentTerm > rnd.currentTerm clause is for PRE_CANDIDATE_REQUEST_VOTE.
		// For CANDIDATE_REQUEST_VOTE, msg.SenderCurrentTerm should always equal rnd.currentTerm.
		ok := msg.SenderCurrentTerm > rnd.currentTerm || rnd.votedFor == NoNodeID || rnd.votedFor == msg.From

		// isUpToDate returns true if the given (index, term) log is more up-to-date
		// than the last entry in the existing logs. It returns true, first if the
		// term is greater than the last term. Second if the index is greater than
		// the last index.
		moreUpToDate := rnd.storageRaftLog.isUpToDate(msg.LogIndex, msg.LogTerm)

		if ok && moreUpToDate {
			raftLogger.Infof("%s received vote-request %s, votes for %s", rnd.describe(), msg.Type, types.ID(msg.From))
			rnd.sendToMailbox(raftpb.Message{
				Type: raftpb.VoteResponseType(msg.Type),
				To:   msg.From,
			})

			if msg.Type == raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE {
				rnd.electionTimeoutElapsedTickNum = 0
				rnd.votedFor = msg.From
			}
		} else {
			raftLogger.Infof("%s received vote-request %s, rejects %s", rnd.describe(), msg.Type, types.ID(msg.From))
			rnd.sendToMailbox(raftpb.Message{
				Type:   raftpb.VoteResponseType(msg.Type),
				To:     msg.From,
				Reject: true,
			})
		}

	default:
		rnd.stepFunc(rnd, msg)
	}
	return nil
}
