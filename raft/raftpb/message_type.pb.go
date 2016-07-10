// Code generated by protoc-gen-gogo.
// source: raft/raftpb/message_type.proto
// DO NOT EDIT!

package raftpb

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import _ "github.com/gogo/protobuf/gogoproto"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// (etcd raftpb.MessageType)
type MESSAGE_TYPE int32

const (
	// INTERNAL_TRIGGER_FOLLOWER_OR_CANDIDATE_TO_START_CAMPAIGN message is used locally by Candidate/Follower
	// to start an election after election timeout.
	//
	// Every server in Raft starts as a follower, and when it has not received
	// any message from a valid leader before the randomized election timeout,
	// it will start an election to become a leader. To begin an election, a
	// follower increments its current term and becomes or remains Candidate.
	// Then votes for itself and send RequestVote RPCs in parallel to other peers.
	// (Raft 3.4 Leader election)
	//
	// It is an internal(local) message that is never sent to other peers over the network.
	//
	// (etcd: raft.raftpb.MsgHup)
	MESSAGE_TYPE_INTERNAL_TRIGGER_FOLLOWER_OR_CANDIDATE_TO_START_CAMPAIGN MESSAGE_TYPE = 0
	// INTERNAL_TRIGGER_LEADER_TO_SEND_HEARTBEAT message is used locally by Leader,
	// to signal the leader to send a LEADER_HEARTBEAT to its followers.
	// It is triggered periodically after heartbeat timeouts.
	//
	// It is an internal(local) message that is never sent to other peers over the network.
	//
	// (etcd: raft.raftpb.MsgBeat)
	MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_TO_SEND_HEARTBEAT MESSAGE_TYPE = 1
	// INTERNAL_TRIGGER_LEADER_TO_CHECK_QUORUM message is used locally by Leader,
	// to check if quorum of cluster (including itself) is active.
	// And if not, the leader steps down to a follower.
	//
	// Quorum is cluster size / 2 + 1.
	//
	// It is an internal(local) message that is never sent to other peers over the network.
	//
	// (etcd: raft.raftpb.MsgCheckQuorum)
	MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_TO_CHECK_QUORUM MESSAGE_TYPE = 2
	// LEADER_HEARTBEAT message is heartbeat from the Leader.
	// It is triggered by INTERNAL_TRIGGER_LEADER_TO_SEND_HEARTBEAT message, after every heartbeat
	// timeout, and is sent to leader's followers. It is same as an empty
	// raftpb.MsgApp, but includes raftpb.Message.Commit information for
	// followers.
	//
	//   idx1 = Leader.Follower.Progress.Match
	//   idx2 = Leader.raftLog.CommittedIndex
	//   Leader.LEADER_HEARTBEAT.SenderCurrentCommittedIndex = min(idx1, idx2)
	//
	// So that the followers can update their CommittedIndex.
	//
	// (etcd: raft.raftpb.MsgHeartbeat)
	MESSAGE_TYPE_LEADER_HEARTBEAT MESSAGE_TYPE = 3
	// RESPONSE_TO_LEADER_HEARTBEAT message is the response from Follower,
	// for leader's LEADER_HEARTBEAT. It does not contain any information.
	// When a leader receives this message from a follower, the leader marks
	// this follower as active, and sends raftpb.MsgApp if needed.
	//
	// (etcd: raft.raftpb.MsgHeartbeatResp)
	MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT MESSAGE_TYPE = 4
	// CANDIDATE_REQUEST_VOTE message is sent by Candiate.
	// It is triggered by an election, and sent with the candidate's
	// information:
	//
	//   Candidate.CANDIDATE_REQUEST_VOTE.LogTerm  = Candidate.raftLog.lastTerm()
	//   Candidate.CANDIDATE_REQUEST_VOTE.LogIndex = Candidate.raftLog.lastIndex()
	//
	// (etcd: raft.raftpb.MsgVote)
	MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE MESSAGE_TYPE = 5
	// RESPONSE_TO_CANDIDATE_REQUEST_VOTE message is the response to CANDIDATE_REQUEST_VOTE.
	//
	//
	// Leader receives CANDIDATE_REQUEST_VOTE from Candidate, and then:
	//
	//   Leader.RESPONSE_TO_CANDIDATE_REQUEST_VOTE.Reject = true
	//
	//
	// Candidate receives CANDIDATE_REQUEST_VOTE from another candidate, and then:
	//
	//   Candidate.RESPONSE_TO_CANDIDATE_REQUEST_VOTE.Reject = true
	//
	//
	// Follower receives CANDIDATE_REQUEST_VOTE from Candidate, and IF:
	//
	//   i) Candidate.Message.LogTerm > Follower.raftLog.lastTerm()
	//   OR
	//   ii) Candidate.Message.LogTerm == Follower.raftLog.lastTerm()
	//       AND
	//       Candidate.Message.LogIndex >= Follower.raftLog.lastIndex()
	//
	//   THEN
	//      Follower.RESPONSE_TO_CANDIDATE_REQUEST_VOTE.To     = Candidate
	//      Follower.RESPONSE_TO_CANDIDATE_REQUEST_VOTE.Reject = false
	//
	//   ELSE
	//      Follower.RESPONSE_TO_CANDIDATE_REQUEST_VOTE.To     = Candidate
	//      Follower.RESPONSE_TO_CANDIDATE_REQUEST_VOTE.Reject = true
	//
	// (etcd: raft.raftpb.MsgVoteResp)
	MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE MESSAGE_TYPE = 6
	// PROPOSAL_TO_LEADER message is used to forward client requests to its leader,
	// because client requests in Raft are processed through the leader.
	// First option is for the server to reject the request and return the
	// endpoint of the leader to the client. Or the server can proxy the client's
	// requests to the leader without rejection, so that clients can talk to any
	// node in the cluster.
	// (Raft 6.2 Routing requests to the leader)
	//
	// For Leader/Candidate, it is an internal(local) message that is never sent
	// to other peers over the network. For Follower, it is forwarded to Leader
	// over the network.
	//
	// It is triggered whenever local node.Propose, only contains:
	//
	//   Leader/Candidate/Follower.PROPOSAL_TO_LEADER.Entries = Entries
	//
	//
	// Leader receives Leader.PROPOSAL_TO_LEADER:
	//
	//   Leader.appendEntry(Leader.PROPOSAL_TO_LEADER.Entries)
	//   Leader.bcastAppend() to followers
	//
	//
	// Candidate receives Candidate.PROPOSAL_TO_LEADER:
	//
	//   Ignore Candidate.PROPOSAL_TO_LEADER
	//   because it means that there is no leader
	//
	//
	// Follower receives Follower.PROPOSAL_TO_LEADER:
	//
	//   i) Ignore Follower.PROPOSAL_TO_LEADER
	//      if there is no known leader
	//
	//   ii) Send Follower.PROPOSAL_TO_LEADER to its Leader
	//
	//       Follower.PROPOSAL_TO_LEADER.To = Leader
	//       Follower.PROPOSAL_TO_LEADER.Entries = Entries
	//
	//
	// (etcd: raft.raftpb.MsgProp)
	MESSAGE_TYPE_PROPOSAL_TO_LEADER MESSAGE_TYPE = 7
	// APPEND_FROM_LEADER message is only sent by Leader.
	//
	//   newLogIndex   = Leader.Follower.Progress.Next
	//   prevLogIndex  = newLogsIndex - 1
	//   prevLogTerm   = Leader.raftLog.term(prevLogIndex)
	//   entries       = Leader.raftLog.entries(newLogIndex, Leader.maxMsgSize)
	//   leaderCommit  = Leader.raftLog.CommittedIndex
	//
	//   Leader.APPEND_FROM_LEADER.SenderCurrentCommittedIndex = leaderCommit
	//   Leader.APPEND_FROM_LEADER.LogIndex                    = prevLogIndex
	//   Leader.APPEND_FROM_LEADER.LogTerm                     = prevLogTerm
	//   Leader.APPEND_FROM_LEADER.Entries                     = entries
	//
	// (etcd: raft.raftpb.MsgApp)
	MESSAGE_TYPE_APPEND_FROM_LEADER MESSAGE_TYPE = 8
	// RESPONSE_TO_APPEND_FROM_LEADER message is the response to Leader by Follower.
	//
	//   i) response to APPEND_FROM_LEADER:
	//
	//      IF
	//         Follower.raftLog.CommittedIndex > Leader.APPEND_FROM_LEADER.LogIndex
	//
	//      THEN
	//         Follower.RESPONSE_TO_APPEND_FROM_LEADER.To       = Leader
	//         Follower.RESPONSE_TO_APPEND_FROM_LEADER.LogIndex = Follower.raftLog.CommittedIndex
	//         Follower.RESPONSE_TO_APPEND_FROM_LEADER.Reject   = false
	//
	//      AND THEN
	//         Leader updates Leader.Follower.Progress
	//
	//      ELSE IF
	//         Leader.APPEND_FROM_LEADER.LogIndex >= Follower.raftLog.CommittedIndex
	//
	//      AND IF
	//         term1 = Leader.APPEND_FROM_LEADER.LogTerm
	//         term2 = Follower.term(Leader.APPEND_FROM_LEADER.LogIndex)
	//         term1 == term2
	//
	//         THEN
	//            idx1 = Leader.APPEND_FROM_LEADER.SenderCurrentCommittedIndex
	//            idx2 = Leader.APPEND_FROM_LEADER.LogIndex + len(new entries)
	//            Follower.raftLog.commitTo(min(idx1, idx2))
	//
	//            Follower.RESPONSE_TO_APPEND_FROM_LEADER.To       = Leader
	//            Follower.RESPONSE_TO_APPEND_FROM_LEADER.LogIndex = idx2
	//            Follower.RESPONSE_TO_APPEND_FROM_LEADER.Reject   = false
	//
	//      ELSE
	//         Follower.RESPONSE_TO_APPEND_FROM_LEADER.To         = Leader
	//         Follower.RESPONSE_TO_APPEND_FROM_LEADER.LogIndex   = Leader.APPEND_FROM_LEADER.LogIndex
	//         Follower.RESPONSE_TO_APPEND_FROM_LEADER.Reject     = true
	//         Follower.RESPONSE_TO_APPEND_FROM_LEADER.RejectHint = Follower.raftLog.lastIndex()
	//
	//         THEN
	//            Leader gets this Rejection and updates its Follower.Progress with:
	//               idx1 = Follower.RESPONSE_TO_APPEND_FROM_LEADER.LogIndex
	//               idx2 = Follower.RESPONSE_TO_APPEND_FROM_LEADER.RejectHint + 1
	//               Leader.Follower.Progress.Next = min(idx1, idx2)
	//
	//   (etcd: raft.*raft.handleAppendEntries)
	//
	//
	//   ii) response to SNAPSHOT_FROM_LEADER
	//
	//      IF
	//         idx1 = Follower.raftLog.CommittedIndex
	//         idx2 = Leader.SNAPSHOT_FROM_LEADER.Index
	//         idx1 >= idx2
	//            THEN false
	//
	//         idx   = Leader.SNAPSHOT_FROM_LEADER.Index
	//         term1 = Follower.raftLog.term(Leader.SNAPSHOT_FROM_LEADER.Index)
	//         term2 = Leader.SNAPSHOT_FROM_LEADER.Term
	//         term1 == term2
	//            THEN Follower.raftLog.commitTo(idx)
	//            AND THEN false
	//
	//      ELSE
	//         true
	//
	//      IF true, THEN successfully recovered the state machine from a snapshot
	//         Follower.RESPONSE_TO_APPEND_FROM_LEADER.To       = Leader
	//         Follower.RESPONSE_TO_APPEND_FROM_LEADER.LogIndex = Follower.raftLog.lastIndex()
	//         Follower.RESPONSE_TO_APPEND_FROM_LEADER.Reject   = false
	//
	//      ELSE ignores snapshot
	//         Follower.RESPONSE_TO_APPEND_FROM_LEADER.To       = Leader
	//         Follower.RESPONSE_TO_APPEND_FROM_LEADER.LogIndex = Follower.raftLog.CommittedIndex
	//         Follower.RESPONSE_TO_APPEND_FROM_LEADER.Reject   = false
	//
	//   (etcd: raft.*raft.handleSnapshot)
	//
	//
	//   iii)
	//      term1 = Follower.Term
	//      term2 = Leader.LEADER_HEARTBEAT.LogTerm
	//      term3 = Leader.APPEND_FROM_LEADER.LogTerm
	//      term1 > term2 || term1 > term3
	//
	//         THEN
	//            Follower received message from Leader with a lower term
	//         SO
	//            Follower ignores this message
	//
	//   (etcd: raft.*raft.Step)
	//
	//
	// (etcd: raft.raftpb.MsgAppResp)
	MESSAGE_TYPE_RESPONSE_TO_APPEND_FROM_LEADER MESSAGE_TYPE = 9
	// SNAPSHOT_FROM_LEADER is only sent by Leader.
	// It is triggered when the Leader tries to replicate its log (sendAppend) but:
	//
	//   i) term, err = Leader.raftLog.term(Leader.Follower.Progress.Next - 1)
	//      err == ErrCompacted
	//
	//   OR
	//
	//   ii) entries, err = Leader.raftLog.entries(Leader.Follower.Progress.Next, Leader.maxMsgSize)
	//       err != nil
	//
	//   THEN
	//      snap = Leader.raftLog.snapshot()
	//      Leader.SNAPSHOT_FROM_LEADER.Snapshot = snap
	//      Leader.Follower.Progress.becomeSnapshot(snap.Index)
	//
	//
	// (etcd: raft.raftpb.MsgSnap)
	MESSAGE_TYPE_SNAPSHOT_FROM_LEADER MESSAGE_TYPE = 10
	// INTERNAL_RESPONSE_TO_SNAPSHOT_FROM_LEADER message is the response to SNAPSHOT_FROM_LEADER from Follower.
	//
	// It is an internal(local) message that is never sent to other peers over the network.
	//
	// (etcd: raft.raftpb.MsgSnapStatus)
	MESSAGE_TYPE_INTERNAL_RESPONSE_TO_SNAPSHOT_FROM_LEADER MESSAGE_TYPE = 11
	// INTERNAL_UNREACHABLE_FOLLOWER message notifies Leader that Follower is not reachable.
	//
	// It is an internal(local) message that is never sent to other peers over the network.
	//
	// (etcd: raft.raftpb.MsgUnreachable)
	MESSAGE_TYPE_INTERNAL_UNREACHABLE_FOLLOWER MESSAGE_TYPE = 12
	// INTERNAL_LEADER_TRANSFER message allows Leader to transfer its leadership to another.
	//
	// It is an internal(local) message that is never sent to other peers over the network.
	//
	// (etcd: raft.raftpb.MsgTransferLeader)
	MESSAGE_TYPE_INTERNAL_LEADER_TRANSFER MESSAGE_TYPE = 13
	// FORCE_ELECTION_TIMEOUT message makes Leader send time-out message to its peers,
	// so that Follower can force election timeouts and start campaigning. Candidate
	// is already campaigning, so it ignores this message. It is used when the leader
	// transfer is happening.
	//
	// (etcd: raft.raftpb.MsgTimeoutNow)
	MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT MESSAGE_TYPE = 14
	// READ_LEADER_CURRENT_COMMITTED_INDEX is used to serve clients' read-only queries without
	// going through Raft, but still with 'quorum-get' on. It bypasses the Raft log, but
	// still preserves the linearizability of reads, with lower costs.
	//
	// If a request goes through Raft log, it needs replication, which requires synchronous
	// disk writes in order to append those request entries to its log. Since read-only requests
	// do not change any state of replicated state machine, these writes can be time- and
	// resource-consuming.
	//
	// To bypass the Raft log with linearizable reads:
	//
	//   1. If Leader has not yet committed an entry from SenderCurrentTerm, it waits until it has done so.
	//
	//   2. Leader saves its SenderCurrentCommittedIndex in a local variable 'readIndex', which is used
	//      as a lower bound for the version of the state that read-only queries operate against.
	//
	//   3. Leader must ensure that it hasn't been superseded by a newer Leader,
	//      by issuing a new round of heartbeats and waiting for responses from cluster quorum.
	//
	//   4. These responses from Followers acknowledging the Leader indicates that
	//      there was no other Leader at the moment Leader sent out heartbeats.
	//
	//   5. Therefore, Leader's 'readIndex' was, at the time, the largest committed index,
	//      ever seen by any node in the cluster.
	//
	//   6. Leader now waits for its state machine to advance at least as far as the 'readIndex'.
	//      And this is current enought to satisfy linearizability.
	//
	//   7. Leader can now respond to those read-only client requests.
	//
	// (Raft 6.4 Processing read-only queries more efficiently, page 72)
	// (etcd: raft.raftpb.MsgReadIndex)
	MESSAGE_TYPE_READ_LEADER_CURRENT_COMMITTED_INDEX MESSAGE_TYPE = 15
	// RESPONSE_TO_READ_LEADER_CURRENT_COMMITTED_INDEX is response to READ_LEADER_CURRENT_COMMITTED_INDEX.
	//
	// (Raft 6.4 Processing read-only queries more efficiently, page 72)
	// (etcd: raft.raftpb.MsgReadIndexResp)
	MESSAGE_TYPE_RESPONSE_TO_READ_LEADER_CURRENT_COMMITTED_INDEX MESSAGE_TYPE = 16
)

var MESSAGE_TYPE_name = map[int32]string{
	0:  "INTERNAL_TRIGGER_FOLLOWER_OR_CANDIDATE_TO_START_CAMPAIGN",
	1:  "INTERNAL_TRIGGER_LEADER_TO_SEND_HEARTBEAT",
	2:  "INTERNAL_TRIGGER_LEADER_TO_CHECK_QUORUM",
	3:  "LEADER_HEARTBEAT",
	4:  "RESPONSE_TO_LEADER_HEARTBEAT",
	5:  "CANDIDATE_REQUEST_VOTE",
	6:  "RESPONSE_TO_CANDIDATE_REQUEST_VOTE",
	7:  "PROPOSAL_TO_LEADER",
	8:  "APPEND_FROM_LEADER",
	9:  "RESPONSE_TO_APPEND_FROM_LEADER",
	10: "SNAPSHOT_FROM_LEADER",
	11: "INTERNAL_RESPONSE_TO_SNAPSHOT_FROM_LEADER",
	12: "INTERNAL_UNREACHABLE_FOLLOWER",
	13: "INTERNAL_LEADER_TRANSFER",
	14: "FORCE_ELECTION_TIMEOUT",
	15: "READ_LEADER_CURRENT_COMMITTED_INDEX",
	16: "RESPONSE_TO_READ_LEADER_CURRENT_COMMITTED_INDEX",
}
var MESSAGE_TYPE_value = map[string]int32{
	"INTERNAL_TRIGGER_FOLLOWER_OR_CANDIDATE_TO_START_CAMPAIGN": 0,
	"INTERNAL_TRIGGER_LEADER_TO_SEND_HEARTBEAT":                1,
	"INTERNAL_TRIGGER_LEADER_TO_CHECK_QUORUM":                  2,
	"LEADER_HEARTBEAT":                                         3,
	"RESPONSE_TO_LEADER_HEARTBEAT":                             4,
	"CANDIDATE_REQUEST_VOTE":                                   5,
	"RESPONSE_TO_CANDIDATE_REQUEST_VOTE":                       6,
	"PROPOSAL_TO_LEADER":                                       7,
	"APPEND_FROM_LEADER":                                       8,
	"RESPONSE_TO_APPEND_FROM_LEADER":                           9,
	"SNAPSHOT_FROM_LEADER":                                     10,
	"INTERNAL_RESPONSE_TO_SNAPSHOT_FROM_LEADER":                11,
	"INTERNAL_UNREACHABLE_FOLLOWER":                            12,
	"INTERNAL_LEADER_TRANSFER":                                 13,
	"FORCE_ELECTION_TIMEOUT":                                   14,
	"READ_LEADER_CURRENT_COMMITTED_INDEX":                      15,
	"RESPONSE_TO_READ_LEADER_CURRENT_COMMITTED_INDEX":          16,
}

func (x MESSAGE_TYPE) String() string {
	return proto.EnumName(MESSAGE_TYPE_name, int32(x))
}
func (MESSAGE_TYPE) EnumDescriptor() ([]byte, []int) { return fileDescriptorMessageType, []int{0} }

func init() {
	proto.RegisterEnum("raftpb.MESSAGE_TYPE", MESSAGE_TYPE_name, MESSAGE_TYPE_value)
}

var fileDescriptorMessageType = []byte{
	// 440 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x8c, 0x92, 0xcf, 0x6e, 0xd3, 0x40,
	0x10, 0xc6, 0x13, 0x28, 0x01, 0x86, 0x02, 0xab, 0x55, 0x54, 0x55, 0x55, 0xb1, 0xa0, 0x48, 0x54,
	0x80, 0x5a, 0x1f, 0x7a, 0xe1, 0xc0, 0x65, 0xb3, 0x9e, 0x24, 0x16, 0xf6, 0xee, 0x76, 0x76, 0xcc,
	0x9f, 0xd3, 0xaa, 0x41, 0x69, 0xe0, 0x50, 0x39, 0x6a, 0xd3, 0x03, 0x6f, 0xc2, 0x23, 0xf5, 0xc8,
	0x23, 0x40, 0x78, 0x0d, 0x0e, 0xc8, 0x2e, 0x49, 0x5d, 0xb5, 0x20, 0x2e, 0x96, 0x67, 0xbe, 0xdf,
	0x37, 0xfe, 0x3c, 0xbb, 0x10, 0x1d, 0x1f, 0x1c, 0xce, 0xe2, 0xea, 0x31, 0x1d, 0xc5, 0x47, 0xe3,
	0x93, 0x93, 0x83, 0xc9, 0x38, 0xcc, 0xbe, 0x4c, 0xc7, 0xbb, 0xd3, 0xe3, 0x72, 0x56, 0xca, 0xce,
	0xb9, 0xb4, 0xb1, 0x33, 0xf9, 0x3c, 0xfb, 0x74, 0x3a, 0xda, 0xfd, 0x58, 0x1e, 0xc5, 0x93, 0x72,
	0x52, 0xc6, 0xb5, 0x3c, 0x3a, 0x3d, 0xac, 0xab, 0xba, 0xa8, 0xdf, 0xce, 0x6d, 0x2f, 0x7e, 0xad,
	0xc0, 0x6a, 0x8e, 0xde, 0xab, 0x01, 0x06, 0xfe, 0xe0, 0x50, 0xbe, 0x86, 0x57, 0xa9, 0x61, 0x24,
	0xa3, 0xb2, 0xc0, 0x94, 0x0e, 0x06, 0x48, 0xa1, 0x6f, 0xb3, 0xcc, 0xbe, 0x43, 0x0a, 0x96, 0x82,
	0x56, 0x26, 0x49, 0x13, 0xc5, 0x18, 0xd8, 0x06, 0xcf, 0x8a, 0x38, 0x68, 0x95, 0x3b, 0x95, 0x0e,
	0x8c, 0x68, 0xc9, 0x1d, 0x78, 0x7e, 0xc5, 0x9d, 0xa1, 0x4a, 0x90, 0x6a, 0x1c, 0x4d, 0x12, 0x86,
	0xa8, 0x88, 0x7b, 0xa8, 0x58, 0xb4, 0xe5, 0x4b, 0xd8, 0xfe, 0x07, 0xae, 0x87, 0xa8, 0xdf, 0x84,
	0xfd, 0xc2, 0x52, 0x91, 0x8b, 0x1b, 0xb2, 0x0b, 0xe2, 0x8f, 0x76, 0x31, 0xe2, 0xa6, 0x7c, 0x0c,
	0x9b, 0x84, 0xde, 0x59, 0xe3, 0xeb, 0x44, 0x57, 0x88, 0x15, 0xb9, 0x01, 0x6b, 0x17, 0xa1, 0x09,
	0xf7, 0x0b, 0xf4, 0x1c, 0xde, 0x5a, 0x46, 0x71, 0x4b, 0x3e, 0x83, 0xad, 0xa6, 0xfb, 0x2f, 0x5c,
	0x47, 0xae, 0x81, 0x74, 0x64, 0x9d, 0xf5, 0x55, 0xd0, 0xc5, 0x57, 0xc4, 0xed, 0xaa, 0xaf, 0x9c,
	0xab, 0x7e, 0xab, 0x4f, 0x36, 0x5f, 0xf4, 0xef, 0xc8, 0x2d, 0x88, 0x9a, 0x73, 0xaf, 0x61, 0xee,
	0xca, 0x75, 0xe8, 0x7a, 0xa3, 0x9c, 0x1f, 0x5a, 0xbe, 0xa4, 0xc0, 0xa5, 0x2d, 0x36, 0xc7, 0x5c,
	0x8b, 0xdf, 0x93, 0x4f, 0xe0, 0xd1, 0x12, 0x2f, 0x0c, 0xa1, 0xd2, 0x43, 0xd5, 0xcb, 0x70, 0x79,
	0x6c, 0x62, 0x55, 0x6e, 0xc2, 0xfa, 0x12, 0x59, 0x2c, 0x98, 0x94, 0xf1, 0x7d, 0x24, 0x71, 0xbf,
	0xda, 0x50, 0xdf, 0x92, 0xc6, 0x80, 0x19, 0x6a, 0x4e, 0xad, 0x09, 0x9c, 0xe6, 0x68, 0x0b, 0x16,
	0x0f, 0xe4, 0x36, 0x3c, 0x25, 0x54, 0xc9, 0xc2, 0xa5, 0x0b, 0x22, 0x34, 0x1c, 0xb4, 0xcd, 0xf3,
	0x94, 0x19, 0x93, 0x90, 0x9a, 0x04, 0xdf, 0x8b, 0x87, 0x72, 0x0f, 0xe2, 0x66, 0xd6, 0xff, 0x31,
	0x89, 0x5e, 0xf7, 0xec, 0x47, 0xd4, 0x3a, 0x9b, 0x47, 0xed, 0x6f, 0xf3, 0xa8, 0xfd, 0x7d, 0x1e,
	0xb5, 0xbf, 0xfe, 0x8c, 0x5a, 0xa3, 0x4e, 0x7d, 0x37, 0xf7, 0x7e, 0x07, 0x00, 0x00, 0xff, 0xff,
	0x70, 0x9e, 0x8d, 0x5a, 0xf4, 0x02, 0x00, 0x00,
}
