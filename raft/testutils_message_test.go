package raft

import (
	"fmt"
	"reflect"

	"github.com/gyuho/db/raft/raftpb"
)

type messageSlice []raftpb.Message

func (s messageSlice) Len() int           { return len(s) }
func (s messageSlice) Less(i, j int) bool { return fmt.Sprint(s[i]) < fmt.Sprint(s[j]) }
func (s messageSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// (etcd raft.acceptAndReply)
func createAppendResponseMessage(msg raftpb.Message) raftpb.Message {
	if msg.Type != raftpb.MESSAGE_TYPE_LEADER_APPEND {
		raftLogger.Panicf("message type expected %q, got %q", raftpb.MESSAGE_TYPE_LEADER_APPEND, msg.Type)
	}
	return raftpb.Message{
		Type:              raftpb.MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND,
		From:              msg.To,
		To:                msg.From,
		SenderCurrentTerm: msg.SenderCurrentTerm,
		LogIndex:          msg.LogIndex + uint64(len(msg.Entries)),
	}
}

// (etcd raft.commitNoopEntry)
func (rnd *raftNode) commitNoopEntry() {
	rnd.assertNodeState(raftpb.NODE_STATE_LEADER)

	rnd.leaderReplicateAppendRequests()

	msgs := rnd.readAndClearMailbox()

	for _, msg := range msgs {
		if msg.Type != raftpb.MESSAGE_TYPE_LEADER_APPEND || len(msg.Entries) != 1 || msg.Entries[0].Data != nil {
			raftLogger.Panicf("expected noop entry, got %+v", msg)
		}
		rnd.Step(createAppendResponseMessage(msg))
	}

	// ignore further messages to refresh followers' commit index
	rnd.readAndClearMailbox()

	st, ok := rnd.storageRaftLog.storageStable.(*StorageStableInMemory)
	if !ok {
		raftLogger.Panicf("expected *StorageStableInMemory, got %v", reflect.TypeOf(rnd.storageRaftLog.storageStable))
	}
	st.Append(rnd.storageRaftLog.unstableEntries()...)

	rnd.storageRaftLog.appliedTo(rnd.storageRaftLog.committedIndex)
	rnd.storageRaftLog.persistedEntriesAt(rnd.storageRaftLog.lastIndex(), rnd.storageRaftLog.lastTerm())
}
