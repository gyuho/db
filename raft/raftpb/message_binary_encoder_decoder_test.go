package raftpb

import (
	"bytes"
	"reflect"
	"testing"
)

// (etcd rafthttp.TestMessage)
func Test_MessageBinaryEncoderDecoder(t *testing.T) {
	tests := []Message{
		{
			Type:              MESSAGE_TYPE_LEADER_APPEND,
			From:              1,
			To:                2,
			SenderCurrentTerm: 1,
			LogIndex:          3,
			LogTerm:           1,
			Entries:           []Entry{{Index: 4, Term: 1}},
		},
		{
			Type: MESSAGE_TYPE_PROPOSAL_TO_LEADER,
			From: 1,
			To:   2,
			Entries: []Entry{
				{Data: []byte("testdata")},
				{Data: []byte("testdata")},
				{Data: []byte("testdata")},
			},
		},
		{Type: MESSAGE_TYPE_LEADER_HEARTBEAT},
	}

	for i, tt := range tests {
		b := &bytes.Buffer{}

		enc := NewMessageBinaryEncoder(b)
		if err := enc.Encode(&tt); err != nil {
			t.Fatalf("#%d: unexpected encode message error: %v", i, err)
		}

		dec := NewMessageBinaryDecoder(b)
		m, err := dec.Decode()
		if err != nil {
			t.Fatalf("#%d: unexpected decode message error: %v", i, err)
		}

		if !reflect.DeepEqual(m, tt) {
			t.Fatalf("#%d: message = %+v, want %+v", i, m, tt)
		}
	}
}
