package rafthttp

import "github.com/gyuho/db/raft/raftpb"

// emptyLeaderHeartbeat is a special heartbeat message without From, To fields.
//
// (etcd rafthttp.linkHeartbeatMessage)
var emptyLeaderHeartbeat = raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT}

// (etcd rafthttp.isLinkHeartbeatMessage)
func isEmptyLeaderHeartbeat(msg raftpb.Message) bool {
	return msg.Type == raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT && msg.From == 0 && msg.To == 0
}
