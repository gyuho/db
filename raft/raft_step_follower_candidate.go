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
		raftLogger.Panicf("tickFuncFollowerElectionTimeout must be called by follower [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
	}

	rnd.electionTimeoutElapsedTickNum++
	if rnd.promotableToLeader() && rnd.pastElectionTimeout() {
		rnd.electionTimeoutElapsedTickNum = 0
		rnd.Step(raftpb.Message{
			Type: raftpb.MESSAGE_TYPE_INTERNAL_TRIGGER_FOLLOWER_OR_CANDIDATE_TO_START_CAMPAIGN,
			From: rnd.id,
		})
	}
}

// (etcd raft.raft.poll)
func (rnd *raftNode) candidateReceivedVoteFrom(fromID uint64, voted bool) int {
	if voted {
		raftLogger.Infof("candidate %x received vote from %x at term %d", rnd.id, fromID, rnd.term)
	} else {
		raftLogger.Infof("candidate %x received vote-rejection from %x at term %d", rnd.id, fromID, rnd.term)
	}

	if _, ok := rnd.votedFrom[fromID]; !ok {
		rnd.votedFrom[fromID] = voted
	} else { // ???
		raftLogger.Panicf("candidate %x received duplicate votes from %x (voted %v)", rnd.id, fromID, voted)
	}

	grantedN := 0
	for _, voted := range rnd.votedFrom {
		if voted {
			grantedN++
		}
	}

	return grantedN
}

// (etcd raft.raft.handleHeartbeat)
func (rnd *raftNode) followerRespondToLeaderHeartbeat(msg raftpb.Message) {
	rnd.storageRaftLog.commitTo(msg.SenderCurrentCommittedIndex)
	rnd.sendToMailbox(raftpb.Message{
		Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT,
		To:   msg.From,
	})
}

// (etcd raft.raft.campaign)
func (rnd *raftNode) followerBecomeCandidateAndStartCampaign() {
	rnd.becomeCandidate()

	// vote for itself, and then if voted from quorum, become leader
	if rnd.quorum() == rnd.candidateReceivedVoteFrom(rnd.id, true) {
		rnd.becomeLeader()
		return
	}

	// request vote from peers
	for id := range rnd.allProgresses {
		if id == rnd.id {
			continue
		}

		raftLogger.Infof(
			"candidate %x [last log index=%d | last log term=%d] is sending vote requests to %x at term %d",
			rnd.id, rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm(), id, rnd.term,
		)
		rnd.sendToMailbox(raftpb.Message{
			Type:     raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE,
			To:       id,
			LogIndex: rnd.storageRaftLog.lastIndex(),
			LogTerm:  rnd.storageRaftLog.lastTerm(),
		})
	}
}

// (etcd raft.raft.handleAppendEntries with raftpb.MsgApp)
func (rnd *raftNode) followerHandleAppendFromLeader(msg raftpb.Message) {
	if rnd.storageRaftLog.committedIndex > msg.LogIndex {
		rnd.sendToMailbox(raftpb.Message{
			Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_APPEND_FROM_LEADER,
			To:       msg.From,
			LogIndex: rnd.storageRaftLog.committedIndex,
		})
		return
	}

	if lastIndex, ok := rnd.storageRaftLog.maybeAppend(msg.LogIndex, msg.LogTerm, msg.SenderCurrentCommittedIndex, msg.Entries...); ok {
		rnd.sendToMailbox(raftpb.Message{
			Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_APPEND_FROM_LEADER,
			To:       msg.From,
			LogIndex: lastIndex,
		})
		return
	}

	raftLogger.Infof("follower %x [log index=%d | log term=%d] rejects appends from leader %x [log index=%d | log term=%d]",
		rnd.id, msg.LogIndex, rnd.storageRaftLog.zeroTermOnErrCompacted(rnd.storageRaftLog.term(msg.LogIndex)), msg.From, msg.LogIndex, msg.LogTerm)
	rnd.sendToMailbox(raftpb.Message{
		Type:     raftpb.MESSAGE_TYPE_RESPONSE_TO_APPEND_FROM_LEADER,
		To:       msg.From,
		LogIndex: msg.LogIndex,
		Reject:   true,
		RejectHintFollowerLogLastIndex: rnd.storageRaftLog.lastIndex(),
	})
}

// (etcd raft.raft.handleSnapshot with raftpb.MsgSnap)
func (rnd *raftNode) followerRestoreSnapshotFromLeader(msg raftpb.Message) {
	if rnd.id == rnd.leaderID {
		raftLogger.Panicf("followerRestoreSnapshotFromLeader must be called by follower [id=%x | leader id=%x]", rnd.id, rnd.leaderID)
	}

	snapMetaIndex, snapMetaTerm := msg.Snapshot.Metadata.Index, msg.Snapshot.Metadata.Term

	raftLogger.Infof("follower %x [committed index=%d] is restoring snapshot from leader %x [index=%d | term=%d]",
		rnd.id, rnd.storageRaftLog.committedIndex, rnd.leaderID, snapMetaIndex, snapMetaTerm)

	if rnd.followerRestoreSnapshot(msg.Snapshot) {
		raftLogger.Infof("follower %x [committed index=%d] restored snapshot from leader %x [index=%d | term=%d]",
			rnd.id, rnd.storageRaftLog.committedIndex, rnd.leaderID, snapMetaIndex, snapMetaTerm)
		rnd.sendToMailbox(raftpb.Message{
			Type:     raftpb.MESSAGE_TYPE_INTERNAL_RESPONSE_TO_SNAPSHOT_FROM_LEADER,
			To:       msg.From,
			LogIndex: rnd.storageRaftLog.lastIndex(),
		})
		return
	}

	raftLogger.Infof("follower %x [committed index=%d] ignored snapshot from leader %x [index=%d | term=%d]",
		rnd.id, rnd.storageRaftLog.committedIndex, rnd.leaderID, snapMetaIndex, snapMetaTerm)
	rnd.sendToMailbox(raftpb.Message{
		Type:     raftpb.MESSAGE_TYPE_INTERNAL_RESPONSE_TO_SNAPSHOT_FROM_LEADER,
		To:       msg.From,
		LogIndex: rnd.storageRaftLog.committedIndex,
	})
}

// followerRestoreSnapshot returns true iff snapshot index is greater than committed index
// and the snapshot contains an entries of log and term that does not exist in the current follower.
//
// (etcd raft.raft.restore)
func (rnd *raftNode) followerRestoreSnapshot(snap raftpb.Snapshot) bool {
	if rnd.storageRaftLog.committedIndex >= snap.Metadata.Index {
		return false
	}

	if rnd.storageRaftLog.matchTerm(snap.Metadata.Index, snap.Metadata.Term) {
		raftLogger.Infof(`

	follower %x [committed index=%d | last index=%d | last term=%d]
	fast-forwarded commit
	from leader snapshot [index=%d | term=%d]

`, rnd.id, rnd.storageRaftLog.committedIndex, rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm(),
			snap.Metadata.Index, snap.Metadata.Term,
		)
		rnd.storageRaftLog.commitTo(snap.Metadata.Index)
		return false
	}

	raftLogger.Infof(`

	follower %x [committed index=%d | last index=%d | last term=%d]
	is restoring snapshot its state
	from leader snapshot [index=%d | term=%d]

`, rnd.id, rnd.storageRaftLog.committedIndex, rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm(),
		snap.Metadata.Index, snap.Metadata.Term,
	)

	rnd.storageRaftLog.restoreSnapshot(snap)

	raftLogger.Infof("follower %x is now initializing its progresses of peers", rnd.id)
	rnd.allProgresses = make(map[uint64]*Progress)
	for _, id := range snap.Metadata.ConfigState.IDs {
		matchIndex := uint64(0)
		if id == rnd.id {
			matchIndex = rnd.storageRaftLog.lastIndex()
		}
		nextIndex := rnd.storageRaftLog.lastIndex() + 1

		rnd.updateProgress(id, matchIndex, nextIndex)
		raftLogger.Infof("follower %x restored progress of %x %s", rnd.id, id, rnd.allProgresses[id])
	}

	return true
}

// (etcd raft.raft.stepFollower)
func stepFollower(rnd *raftNode, msg raftpb.Message) {
	switch msg.Type {
	case raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER: // pb.MsgProp
		if rnd.leaderID == NoNodeID {
			raftLogger.Infof("follower %x has no leader at term %d (dropping proposal)", rnd.id, rnd.leaderID, rnd.term)
			return
		}
		msg.To = rnd.leaderID
		rnd.sendToMailbox(msg)

	case raftpb.MESSAGE_TYPE_APPEND_FROM_LEADER: // pb.MsgApp
		rnd.electionTimeoutElapsedTickNum = 0
		rnd.leaderID = msg.From
		rnd.followerHandleAppendFromLeader(msg)

	case raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT: // pb.MsgHeartbeat
		rnd.electionTimeoutElapsedTickNum = 0
		rnd.leaderID = msg.From
		rnd.followerRespondToLeaderHeartbeat(msg)

	case raftpb.MESSAGE_TYPE_SNAPSHOT_FROM_LEADER: // pb.MsgSnap
		rnd.electionTimeoutElapsedTickNum = 0
		rnd.leaderID = msg.From // ??? why not in etcd
		rnd.followerRestoreSnapshotFromLeader(msg)

	case raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE: // pb.MsgVote
		// isUpToDate returns true if the given (index, term) log is more up-to-date
		// than the last entry in the existing logs. It returns true, first if the
		// term is greater than the last term. Second if the index is greater than
		// the last index.
		if (rnd.votedFor == NoNodeID || rnd.votedFor == msg.From) && rnd.storageRaftLog.isUpToDate(msg.LogIndex, msg.LogTerm) {
			rnd.electionTimeoutElapsedTickNum = 0
			rnd.votedFor = msg.From
			rnd.sendToMailbox(raftpb.Message{
				Type: raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE,
				To:   msg.From,
			})
			raftLogger.Infof("follower %x [log index=%d | log term=%d] voted for %x [log index=%d | log term=%d] at term %d",
				rnd.id, rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm(), rnd.votedFor, msg.LogIndex, msg.LogTerm, rnd.term)
		} else {
			rnd.sendToMailbox(raftpb.Message{
				Type:   raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE,
				To:     msg.From,
				Reject: true,
			})
			raftLogger.Infof("follower %x [log index=%d | log term=%d] rejected to vote for %x [log index=%d | log term=%d] at term %d",
				rnd.id, rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm(), rnd.votedFor, msg.LogIndex, msg.LogTerm, rnd.term)
		}

	case raftpb.MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT: // pb.MsgTimeoutNow
		raftLogger.Infof("follower %x [log term=%d] has received %q, so starts campaign to get leadership", rnd.id, rnd.term, msg.Type)
		rnd.followerBecomeCandidateAndStartCampaign()

	case raftpb.MESSAGE_TYPE_READ_LEADER_CURRENT_COMMITTED_INDEX: // pb.MsgReadIndex
		if rnd.leaderID == NoNodeID {
			raftLogger.Infof("follower %x has no leader at term %d (dropping read index message)", rnd.id, rnd.term)
			return
		}
		msg.To = rnd.leaderID
		rnd.sendToMailbox(msg)

	case raftpb.MESSAGE_TYPE_RESPONSE_TO_READ_LEADER_CURRENT_COMMITTED_INDEX: // pb.MsgReadIndexResp
		if len(msg.Entries) != 1 {
			raftLogger.Errorf("follower %x has received invalid ReadIndex response from %x (entries count %d)", rnd.id, msg.From, len(msg.Entries))
			return
		}

		rnd.readState.Index = msg.LogIndex
		rnd.readState.Data = msg.Entries[0].Data
	}
}

// (etcd raft.raft.becomeFollower)
func (rnd *raftNode) becomeFollower(term, leaderID uint64) {
	rnd.state = raftpb.NODE_STATE_FOLLOWER
	rnd.resetWithTerm(term)
	rnd.leaderID = leaderID

	rnd.stepFunc = stepFollower
	rnd.tickFunc = rnd.tickFuncFollowerElectionTimeout

	raftLogger.Infof("%x has just become follower at term %d [leader id=%x]", rnd.id, term, leaderID)
}

// (etcd raft.raft.stepCandidate)
func stepCandidate(rnd *raftNode, msg raftpb.Message) {
	switch msg.Type {
	case raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER: // pb.MsgProp

	case raftpb.MESSAGE_TYPE_APPEND_FROM_LEADER: // pb.MsgApp

	case raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT: // pb.MsgHeartbeat

	case raftpb.MESSAGE_TYPE_SNAPSHOT_FROM_LEADER: // pb.MsgSnap

	case raftpb.MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE: // pb.MsgVote

	case raftpb.MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE: // pb.MsgVoteResp

	case raftpb.MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT: // pb.MsgTimeoutNow

	}
}

// (etcd raft.raft.becomeCandidate)
func (rnd *raftNode) becomeCandidate() {

	rnd.state = raftpb.NODE_STATE_CANDIDATE
	rnd.resetWithTerm(rnd.term + 1)

	rnd.stepFunc = stepCandidate
	rnd.tickFunc = rnd.tickFuncFollowerElectionTimeout

	rnd.votedFor = rnd.id // vote for itself

	raftLogger.Infof("%x has just become candidate at term %d", rnd.id, rnd.term)
}
