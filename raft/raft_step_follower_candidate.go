package raft

import (
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
)

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
		raftLogger.Panicf("MUST NOT BE called by leader(%s)", types.ID(rnd.id))
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
func (rnd *raftNode) candidateReceivedVoteFrom(fromID uint64, mt raftpb.MESSAGE_TYPE, voted bool) int {
	if voted {
		raftLogger.Infof("%s received vote %s from %s", rnd.describe(), mt, types.ID(fromID))
	} else {
		raftLogger.Infof("%s received vote-rejection %s from %s", rnd.describe(), mt, types.ID(fromID))
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
		Type:    raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT,
		To:      msg.From,
		Context: msg.Context,
	})
}

// (etcd raft.raft.campaign)
func (rnd *raftNode) doCampaign(tp raftpb.CAMPAIGN_TYPE) {
	var voteMsgType raftpb.MESSAGE_TYPE
	var termInVoteRequest uint64
	if tp == raftpb.CAMPAIGN_TYPE_PRE_LEADER_ELECTION {
		rnd.becomePreCandidate()
		voteMsgType = raftpb.MESSAGE_TYPE_PRE_CANDIDATE_REQUEST_VOTE
		// PreVote RPCs are sent for the next term before we've incremented r.Term.
		termInVoteRequest = rnd.currentTerm + 1
	} else {
		rnd.becomeCandidate()
		voteMsgType = raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE
		termInVoteRequest = rnd.currentTerm
	}

	// vote for itself, and then if voted from quorum, become leader
	if rnd.quorum() == rnd.candidateReceivedVoteFrom(rnd.id, voteMsgType, true) {
		// We won the election after voting for ourselves (which must mean that
		// this is a single-node cluster). Advance to the next state.
		if tp == raftpb.CAMPAIGN_TYPE_PRE_LEADER_ELECTION {
			rnd.doCampaign(raftpb.CAMPAIGN_TYPE_LEADER_ELECTION)
		} else {
			rnd.becomeLeader()
		}
		return
	}

	// campaign; request votes from peers
	for id := range rnd.allProgresses {
		if id == rnd.id {
			continue
		}
		raftLogger.Infof("%s sent %s to %s (campaign %q)", rnd.describe(), voteMsgType, types.ID(id), tp)

		var ctx []byte
		if tp == raftpb.CAMPAIGN_TYPE_LEADER_TRANSFER {
			ctx = []byte(tp.String())
		}
		rnd.sendToMailbox(raftpb.Message{
			Type:              voteMsgType,
			To:                id,
			SenderCurrentTerm: termInVoteRequest,
			LogIndex:          rnd.storageRaftLog.lastIndex(),
			LogTerm:           rnd.storageRaftLog.lastTerm(),
			Context:           ctx,
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

	raftLogger.Infof("%s rejects append-request from leader %s [hint=%d]", rnd.describe(), types.ID(msg.From), rnd.storageRaftLog.lastIndex())
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
		raftLogger.Panicf("MUST NOT BE called by leader(%s)", types.ID(rnd.id))
	}

	raftLogger.Infof("%s received %q from leader %s", rnd.describe(), msg.Type, types.ID(msg.From))
	if rnd.restoreSnapshot(msg.Snapshot) {
		raftLogger.Infof("%s finished restoreSnapshot from leader %s", rnd.describe(), types.ID(msg.From))
		rnd.sendToMailbox(raftpb.Message{
			// (X) raftpb.MESSAGE_TYPE_INTERNAL_RESPONSE_TO_LEADER_SNAPSHOT,
			Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND,
			To:       msg.From,
			LogIndex: rnd.storageRaftLog.lastIndex(),
		})
		return
	}

	raftLogger.Infof("%s ignores snapshot from leader %s", rnd.describe(), types.ID(msg.From))
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
		raftLogger.Infof("%s fast-forwards commit from leader snapshot [index=%d | term=%d]", rnd.describe(), snap.Metadata.Index, snap.Metadata.Term)
		rnd.storageRaftLog.commitTo(snap.Metadata.Index)
		return false
	}

	raftLogger.Infof("%s restores its snapshot state from leader snapshot [index=%d | term=%d]", rnd.describe(), snap.Metadata.Index, snap.Metadata.Term)
	rnd.storageRaftLog.restoreSnapshot(snap)

	raftLogger.Infof("%s updates progresses", rnd.describe())
	rnd.allProgresses = make(map[uint64]*Progress)

	for _, id := range snap.Metadata.ConfigState.IDs {
		matchIndex := uint64(0)
		if id == rnd.id {
			matchIndex = rnd.storageRaftLog.lastIndex()
		}
		nextIndex := rnd.storageRaftLog.lastIndex() + 1
		rnd.updateProgress(id, matchIndex, nextIndex)
		raftLogger.Infof("%s updates progress; %s %s", rnd.describe(), types.ID(id), rnd.allProgresses[id])
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

	raftLogger.Infof("%s transitioned from %q", rnd.describe(), oldState)
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
	case raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER:
		if rnd.leaderID == NoNodeID {
			raftLogger.Infof("%s has no leader; dropping %q", rnd.describe(), rnd.leaderID, msg.Type)
			return
		}
		msg.To = rnd.leaderID
		rnd.sendToMailbox(msg)

	case raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT:
		rnd.electionTimeoutElapsedTickNum = 0
		rnd.leaderID = msg.From
		rnd.followerHandleLeaderHeartbeat(msg)

	case raftpb.MESSAGE_TYPE_LEADER_APPEND:
		rnd.electionTimeoutElapsedTickNum = 0
		rnd.leaderID = msg.From
		rnd.followerHandleLeaderAppend(msg)

	case raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT:
		rnd.electionTimeoutElapsedTickNum = 0
		rnd.leaderID = msg.From
		rnd.followerHandleLeaderSnapshot(msg)

	case raftpb.MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT:
		// Leadership transfers never use pre-vote even if r.preVote is true; we
		// know we are not recovering from a partition so there is no need for the
		// extra round trip.
		raftLogger.Infof("%s received %q; start campaign", rnd.describe(), msg.Type)
		rnd.doCampaign(raftpb.CAMPAIGN_TYPE_LEADER_TRANSFER)

	case raftpb.MESSAGE_TYPE_TRANSFER_LEADER:
		if rnd.leaderID == NoNodeID {
			raftLogger.Infof("%s has no leader (dropping %q)", rnd.describe(), msg.Type)
			return
		}
		msg.To = rnd.leaderID
		rnd.sendToMailbox(msg)

	case raftpb.MESSAGE_TYPE_TRIGGER_READ_INDEX:
		if rnd.leaderID == NoNodeID {
			raftLogger.Infof("%s has no leader (dropping %q)", rnd.describe(), msg.Type)
			return
		}
		msg.To = rnd.leaderID
		rnd.sendToMailbox(msg)

	case raftpb.MESSAGE_TYPE_READ_INDEX_DATA:
		if len(msg.Entries) != 1 {
			raftLogger.Errorf("%s got invalid read-index data from %s (entries count %d)", rnd.describe(), types.ID(msg.From), len(msg.Entries))
			return
		}
		rnd.readStates = append(rnd.readStates, ReadState{
			Index:      msg.LogIndex,
			RequestCtx: msg.Entries[0].Data,
		})

	default:
		raftLogger.Infof("%s ignores %q from %s", rnd.describe(), msg.Type, types.ID(msg.From))
	}
}

// (etcd raft.raft.becomePreCandidate)
func (rnd *raftNode) becomePreCandidate() {
	// leader cannot transition to candidate
	rnd.assertUnexpectedNodeState(raftpb.NODE_STATE_LEADER)

	oldState := rnd.state

	// Becoming a pre-candidate changes our step functions and state,
	// but doesn't change anything else. In particular it does not increase
	// r.Term or change r.Vote.
	rnd.state = raftpb.NODE_STATE_PRE_CANDIDATE

	rnd.stepFunc = stepCandidate
	rnd.tickFunc = rnd.tickFuncFollowerElectionTimeout

	raftLogger.Infof("%s transitioned from %q", rnd.describe(), oldState)
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

	raftLogger.Infof("%s transitioned from %q", rnd.describe(), oldState)
}

// (etcd raft.raft.stepCandidate)
func stepCandidate(rnd *raftNode, msg raftpb.Message) {
	// Only handle vote responses corresponding to our candidacy (while in
	// StateCandidate, we may get stale MsgPreVoteResp messages in this term from
	// our pre-candidate state).
	var myVoteRespType raftpb.MESSAGE_TYPE
	if rnd.state == raftpb.NODE_STATE_PRE_CANDIDATE {
		myVoteRespType = raftpb.MESSAGE_TYPE_RESPONSE_TO_PRE_CANDIDATE_REQUEST_VOTE
	} else {
		myVoteRespType = raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE
	}

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
		raftLogger.Infof("%s is dropping %q", rnd.describe(), msg.Type)
		return

	case raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT: // pb.MsgHeartbeat
		raftLogger.Infof("%s becomes follower; received %q from leader %s", rnd.describe(), msg.Type, types.ID(msg.From))
		rnd.becomeFollower(rnd.currentTerm, msg.From)
		rnd.followerHandleLeaderHeartbeat(msg)

	case raftpb.MESSAGE_TYPE_LEADER_APPEND: // pb.MsgApp
		raftLogger.Infof("%s becomes follower; received %q from leader %s", rnd.describe(), msg.Type, types.ID(msg.From))
		rnd.becomeFollower(rnd.currentTerm, msg.From)
		rnd.followerHandleLeaderAppend(msg)

	case raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT: // pb.MsgSnap
		raftLogger.Infof("%s becomes follower; received %q from leader %s", rnd.describe(), msg.Type, types.ID(msg.From))
		rnd.becomeFollower(msg.SenderCurrentTerm, msg.From)
		rnd.followerHandleLeaderSnapshot(msg)

	case myVoteRespType:
		grantedNum := rnd.candidateReceivedVoteFrom(msg.From, msg.Type, !msg.Reject)
		rejectedNum := len(rnd.votedFrom) - grantedNum
		raftLogger.Infof("%s received %s [granted num=%d | rejected num=%d | quorum=%d]", rnd.describe(), msg.Type, grantedNum, rejectedNum, rnd.quorum())

		switch rnd.quorum() {
		case grantedNum:
			if rnd.state == raftpb.NODE_STATE_PRE_CANDIDATE {
				rnd.doCampaign(raftpb.CAMPAIGN_TYPE_LEADER_ELECTION)
			} else {
				rnd.becomeLeader()
				rnd.leaderReplicateAppendRequests()
			}
		case rejectedNum:
			rnd.becomeFollower(rnd.currentTerm, NoNodeID)
		}

	case raftpb.MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT: // pb.MsgTimeoutNow
		raftLogger.Infof("%s ignored %q", rnd.describe(), msg.Type)

	default:
		raftLogger.Warningf("%s ignores %q from %s", rnd.describe(), msg.Type, types.ID(msg.From))
	}
}
