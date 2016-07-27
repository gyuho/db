package raft

import "github.com/gyuho/db/raft/raftpb"

// promotableToLeader return true if the local state machine can be promoted to leader.
//
// (etcd raft.raft.promotable)
func (rnd *raftNode) promotableToLeader() bool {
	_, ok := rnd.allProgresses[rnd.id]
	return ok
}

// tickFuncFollowerElectionTimeout triggers an internal message to leader,
// so that leader can send out heartbeats to its followers.
//
// (etcd raft.raft.tickElection)
func (rnd *raftNode) tickFuncFollowerElectionTimeout() {
	if rnd.id == rnd.leaderID {
		raftLogger.Panicf("MUST NOT BE called by the leader(%x), but called by %q(%x)", rnd.leaderID, rnd.state, rnd.id)
	}

	rnd.electionTimeoutElapsedTickNum++

	if rnd.promotableToLeader() && rnd.pastElectionTimeout() {
		rnd.electionTimeoutElapsedTickNum = 0
		rnd.Step(raftpb.Message{
			Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN,
			From: rnd.id,
		})
	}
}

// (etcd raft.raft.poll)
func (rnd *raftNode) candidateReceivedVoteFrom(fromID uint64, voted bool) int {
	if voted {
		raftLogger.Infof("%s received vote from %x", rnd.describe(), fromID)
	} else {
		raftLogger.Infof("%s received vote-rejection from %x", rnd.describe(), fromID)
	}

	rnd.votedFrom[fromID] = voted

	grantedN := 0
	for _, voted := range rnd.votedFrom {
		if voted {
			grantedN++
		}
	}

	return grantedN
}

// (etcd raft.raft.handleHeartbeat)
func (rnd *raftNode) followerHandleLeaderHeartbeat(msg raftpb.Message) {
	rnd.storageRaftLog.commitTo(msg.SenderCurrentCommittedIndex)
	rnd.sendToMailbox(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT,
		To:   msg.From,
	})
}

// (etcd raft.raft.campaign)
func (rnd *raftNode) becomeCandidateAndCampaign(tp raftpb.CAMPAIGN_TYPE) {
	rnd.becomeCandidate()

	// vote for itself, and then if voted from quorum, become leader
	if rnd.quorum() == rnd.candidateReceivedVoteFrom(rnd.id, true) {
		rnd.becomeLeader()
		return
	}

	// campaign; request votes from peers
	for id := range rnd.allProgresses {
		if id == rnd.id {
			continue
		}

		raftLogger.Infof(`

	%s
	SENDS VOTE-REQUEST to %x
	(campaign type %q)

`, rnd.describeLong(), id, tp)

		var ctx []byte
		if tp == raftpb.CAMPAIGN_TYPE_LEADER_TRANSFER {
			ctx = []byte(tp.String())
		}
		rnd.sendToMailbox(raftpb.Message{
			Type:     raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE,
			To:       id,
			LogIndex: rnd.storageRaftLog.lastIndex(),
			LogTerm:  rnd.storageRaftLog.lastTerm(),
			Context:  ctx,
		})
	}
}

// (etcd raft.raft.handleAppendEntries)
func (rnd *raftNode) followerHandleLeaderAppend(msg raftpb.Message) {
	if rnd.storageRaftLog.committedIndex > msg.LogIndex { // node's committed index is already greater
		// respond with msg.LogIndex so that leader can update the progress
		rnd.sendToMailbox(raftpb.Message{
			Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND,
			To:       msg.From,
			LogIndex: rnd.storageRaftLog.committedIndex,
		})
		return
	}

	if lastIndex, ok := rnd.storageRaftLog.maybeAppend(msg.LogIndex, msg.LogTerm, msg.SenderCurrentCommittedIndex, msg.Entries...); ok {
		rnd.sendToMailbox(raftpb.Message{
			Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND,
			To:       msg.From,
			LogIndex: lastIndex,
		})
		return
	}

	raftLogger.Infof(`

	%s
	RECEIVED %s
	(GOT APPEND-REQUEST FROM LEADER, and REJECTS with hint %d)

`, rnd.describeLong(), raftpb.DescribeMessageLong(msg), rnd.storageRaftLog.lastIndex())

	rnd.sendToMailbox(raftpb.Message{
		Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND,
		To:       msg.From,
		LogIndex: msg.LogIndex,
		Reject:   true,
		RejectHintFollowerLogLastIndex: rnd.storageRaftLog.lastIndex(),
	})
}

// (etcd raft.raft.handleSnapshot)
func (rnd *raftNode) followerHandleLeaderSnapshot(msg raftpb.Message) {
	if rnd.id == rnd.leaderID {
		raftLogger.Panicf("MUST NOT BE called by the leader(%x), but called by %q %x", rnd.leaderID, rnd.state, rnd.id)
	}

	raftLogger.Infof(`

	%s
	RECEIVED %s
	(GOT SNAPSHOT FROM LEADER, and RESTORES SNAPSHOT)

`, rnd.describeLong(), raftpb.DescribeMessageLong(msg))

	if rnd.restoreSnapshot(msg.Snapshot) {

		raftLogger.Infof(`

	%s
	FINISHED RESTORATION FROM LEADER SNAPSHOT
	(sending snapshot RESPONSE to LEADER)

`, rnd.describeLong())

		rnd.sendToMailbox(raftpb.Message{
			// (X) raftpb.MESSAGE_TYPE_INTERNAL_RESPONSE_TO_LEADER_SNAPSHOT,
			Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND,
			To:       msg.From,
			LogIndex: rnd.storageRaftLog.lastIndex(),
		})
		return
	}

	raftLogger.Infof(`

	%s
	IGNORED LEADER SNAPSHOT
	(sending snapshot RESPONSE to LEADER)

`, rnd.describeLong())

	rnd.sendToMailbox(raftpb.Message{
		// (X) raftpb.MESSAGE_TYPE_INTERNAL_RESPONSE_TO_LEADER_SNAPSHOT,
		Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND,
		To:       msg.From,
		LogIndex: rnd.storageRaftLog.committedIndex,
	})
}

// restoreSnapshot returns true iff snapshot index is greater than committed index
// and the snapshot contains an entries of log and term that does not exist in the current follower.
//
// (etcd raft.raft.restore)
func (rnd *raftNode) restoreSnapshot(snap raftpb.Snapshot) bool {
	if rnd.storageRaftLog.committedIndex >= snap.Metadata.Index {
		return false
	}

	if rnd.storageRaftLog.matchTerm(snap.Metadata.Index, snap.Metadata.Term) {
		raftLogger.Infof(`

	%s
	FAST-FORWARDS COMMIT
	FROM LEADER SNAPSHOT [index=%d | term=%d]

`,
			rnd.describeLong(),
			snap.Metadata.Index, snap.Metadata.Term,
		)

		rnd.storageRaftLog.commitTo(snap.Metadata.Index)
		return false
	}

	raftLogger.Infof(`

	%s
	RESTORES ITS SNAPSHOT STATE
	FROM LEADER SNAPSHOT [index=%d | term=%d]

`, rnd.describeLong(), snap.Metadata.Index, snap.Metadata.Term)

	rnd.storageRaftLog.restoreSnapshot(snap)

	raftLogger.Infof("%s initializes its progresses", rnd.describe())
	rnd.allProgresses = make(map[uint64]*Progress)
	for _, id := range snap.Metadata.ConfigState.IDs {
		matchIndex := uint64(0)
		if id == rnd.id {
			matchIndex = rnd.storageRaftLog.lastIndex()
		}

		nextIndex := rnd.storageRaftLog.lastIndex() + 1

		rnd.updateProgress(id, matchIndex, nextIndex)
		raftLogger.Infof("%s restored progress of %x %s", rnd.describe(), id, rnd.allProgresses[id])
	}

	return true
}

// (etcd raft.raft.becomeFollower)
func (rnd *raftNode) becomeFollower(term, leaderID uint64) {
	oldState := rnd.state
	rnd.state = raftpb.NODE_STATE_FOLLOWER
	rnd.resetWithTerm(term)
	rnd.leaderID = leaderID

	rnd.stepFunc = stepFollower
	rnd.tickFunc = rnd.tickFuncFollowerElectionTimeout

	raftLogger.Infof("%s just transitioned from %q", rnd.describe(), oldState)
}

// (etcd raft.raft.stepFollower)
func stepFollower(rnd *raftNode, msg raftpb.Message) {
	rnd.assertNodeState(raftpb.NODE_STATE_FOLLOWER)

	// (Raft §3.4 Leader election, p.16)
	//
	// Follower remains as follower as long as it receives messages from a valid leader.
	// If a follower receives no messages within election timeout from a valid leader,
	// it assumes that there is no viable leader and starts an election.

	switch msg.Type {
	case raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER: // pb.MsgProp
		if rnd.leaderID == NoNodeID {
			raftLogger.Infof("%s has no leader (dropping proposal)", rnd.describe(), rnd.leaderID)
			return
		}
		msg.To = rnd.leaderID
		rnd.sendToMailbox(msg)

	case raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT: // pb.MsgHeartbeat
		rnd.electionTimeoutElapsedTickNum = 0
		rnd.leaderID = msg.From
		rnd.followerHandleLeaderHeartbeat(msg)

	case raftpb.MESSAGE_TYPE_LEADER_APPEND: // pb.MsgApp
		rnd.electionTimeoutElapsedTickNum = 0
		rnd.leaderID = msg.From
		rnd.followerHandleLeaderAppend(msg)

	case raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT: // pb.MsgSnap
		rnd.electionTimeoutElapsedTickNum = 0
		rnd.leaderID = msg.From
		rnd.followerHandleLeaderSnapshot(msg)

	case raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE: // pb.MsgVote
		// isUpToDate returns true if the given (index, term) log is more up-to-date
		// than the last entry in the existing logs. It returns true, first if the
		// term is greater than the last term. Second if the index is greater than
		// the last index.
		if (rnd.votedFor == NoNodeID || rnd.votedFor == msg.From) && rnd.storageRaftLog.isUpToDate(msg.LogIndex, msg.LogTerm) {

			rnd.electionTimeoutElapsedTickNum = 0
			rnd.votedFor = msg.From

			raftLogger.Infof(`

	%s
	RECEIVED %s
	(GOT VOTE-REQUEST FROM, and VOTES FOR %x)

`, rnd.describeLong(), raftpb.DescribeMessageLong(msg), msg.From)

			rnd.sendToMailbox(raftpb.Message{
				Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE,
				To:   msg.From,
			})

		} else {

			raftLogger.Infof(`

	%s
	RECEIVED %s
	(GOT VOTE-REQUEST FROM, and REJECT-VOTES TO %x)

`, rnd.describeLong(), raftpb.DescribeMessageLong(msg), msg.From)

			rnd.sendToMailbox(raftpb.Message{
				Type:   raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE,
				To:     msg.From,
				Reject: true,
			})

		}

	case raftpb.MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT: // pb.MsgTimeoutNow
		raftLogger.Infof("%s received %q, so now campaign to become leader", rnd.describe(), msg.Type)
		rnd.becomeCandidateAndCampaign(raftpb.CAMPAIGN_TYPE_LEADER_TRANSFER)

	case raftpb.MESSAGE_TYPE_TRANSFER_LEADER: // pb.MsgTransferLeader
		if rnd.leaderID == NoNodeID {
			raftLogger.Infof("%s has no leader (dropping %q)", rnd.describe(), msg.Type)
			return
		}
		msg.To = rnd.leaderID
		rnd.sendToMailbox(msg)

	case raftpb.MESSAGE_TYPE_READ_INDEX: // pb.MsgReadIndex
		if rnd.leaderID == NoNodeID {
			raftLogger.Infof("%s has no leader (dropping %q)", rnd.describe(), msg.Type)
			return
		}
		msg.To = rnd.leaderID
		rnd.sendToMailbox(msg)

	case raftpb.MESSAGE_TYPE_RESPONSE_TO_READ_INDEX: // pb.MsgReadIndexResp
		if len(msg.Entries) != 1 {
			raftLogger.Errorf("%s received invalid ReadIndex response from %x (entries count %d)", rnd.describe(), msg.From, len(msg.Entries))
			return
		}

		rnd.readState.Index = msg.LogIndex
		rnd.readState.RequestCtx = msg.Entries[0].Data

	default:
		raftLogger.Infof("%s ignores %q from %x", rnd.describe(), msg.Type, msg.From)
	}
}

// (etcd raft.raft.becomeCandidate)
func (rnd *raftNode) becomeCandidate() {
	// leader cannot transition to candidate
	rnd.assertUnexpectedNodeState(raftpb.NODE_STATE_LEADER)

	oldState := rnd.state

	rnd.state = raftpb.NODE_STATE_CANDIDATE
	rnd.resetWithTerm(rnd.currentTerm + 1)

	rnd.stepFunc = stepCandidate
	rnd.tickFunc = rnd.tickFuncFollowerElectionTimeout

	rnd.votedFor = rnd.id // vote for itself

	raftLogger.Infof("%s just transitioned from %q", rnd.describe(), oldState)
}

// (etcd raft.raft.stepCandidate)
func stepCandidate(rnd *raftNode, msg raftpb.Message) {
	rnd.assertNodeState(raftpb.NODE_STATE_CANDIDATE)

	// (Raft §3.4 Leader election, p.16)
	//
	// To begin an election, a follower increments its current term and becomes candidate.
	// It then votes for itself and sends vote-requests to its peers. Candidate wins the
	// election if it gets quorum of votes in the same term.
	//
	// Each node can vote for at most one candidate in the given term (vote for first-come).
	// And at most one candidate can win the election in the given term.
	//
	//
	// (Raft §3.6.1 Election restriction, p.22)
	//
	// Raft uses voting process to prevent a candidate from being elected if the candidate
	// does not contain all committed entries. Candidate must contact quorum, and the voters
	// rejects the vote-requests from this candidate if its own log is more up-to-date, by
	// comparing the index and term.

	switch msg.Type {
	case raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER: // pb.MsgProp
		raftLogger.Infof("%s so there's no known leader (dropping proposal)", rnd.describe())
		return

	case raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT: // pb.MsgHeartbeat
		raftLogger.Infof("%s steps down to %q, because it received %q from leader %x", rnd.describe(), raftpb.NODE_STATE_FOLLOWER, msg.Type, msg.From)
		rnd.becomeFollower(rnd.currentTerm, msg.From)
		rnd.followerHandleLeaderHeartbeat(msg)

	case raftpb.MESSAGE_TYPE_LEADER_APPEND: // pb.MsgApp
		raftLogger.Infof("%s steps down to %q, because it received %q from leader %x", rnd.describe(), raftpb.NODE_STATE_FOLLOWER, msg.Type, msg.From)
		rnd.becomeFollower(rnd.currentTerm, msg.From)
		rnd.followerHandleLeaderAppend(msg)

	case raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT: // pb.MsgSnap
		raftLogger.Infof("%s steps down to %q, because it received %q from leader %x", rnd.describe(), raftpb.NODE_STATE_FOLLOWER, msg.Type, msg.From)
		rnd.becomeFollower(msg.SenderCurrentTerm, msg.From)
		rnd.followerHandleLeaderSnapshot(msg)

	case raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE: // pb.MsgVote

		// (Raft §3.4 Leader election, p.17)
		//
		// Raft randomizes election timeouts to minimize split votes or two candidates.
		// And each server votes for candidate on first-come base.
		// This randomized retry approach elects a leader rapidly, more obvious and understandable.
		//
		// leader or candidate rejects vote-requests from another server.

		raftLogger.Infof(`

	%s
	REJECTED %s
	(GOT VOTE-REQUEST FROM, and REJECT-VOTED FOR %x)

`, rnd.describeLong(), raftpb.DescribeMessageLong(msg), msg.From)

		rnd.sendToMailbox(raftpb.Message{
			Type:   raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE,
			To:     msg.From,
			Reject: true,
		})

	case raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE: // pb.MsgVoteResp
		grantedNum := rnd.candidateReceivedVoteFrom(msg.From, !msg.Reject)
		rejectedNum := len(rnd.votedFrom) - grantedNum
		raftLogger.Infof("%s [granted num=%d | rejected num=%d | quorum=%d]", rnd.describe(), grantedNum, rejectedNum, rnd.quorum())

		switch rnd.quorum() {
		case grantedNum:
			rnd.becomeLeader()
			rnd.leaderReplicateAppendRequests()

		case rejectedNum:
			rnd.becomeFollower(rnd.currentTerm, NoNodeID)
		}

	case raftpb.MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT: // pb.MsgTimeoutNow
		raftLogger.Infof("%s ignored %q", rnd.describe(), msg.Type)

	default:
		raftLogger.Warningf("%s ignores %q from %x", rnd.describe(), msg.Type, msg.From)
	}
}
