package raftsnap

import (
	"io"

	"github.com/gyuho/db/pkg/ioutil"
	"github.com/gyuho/db/raft/raftpb"
)

// Message is a struct that contains a raft Message and a ReadCloser. The type
// of raft message MUST be MsgSnap, which contains the raft meta-data and an
// additional data []byte field that contains the snapshot of the actual state
// machine.
//
// Message contains the ReadCloser field for handling large snapshot. This avoid
// copying the entire snapshot into a byte array, which consumes a lot of memory.
//
// User of Message should close the Message after sending it.
//
// (etcd snap.Message)
type Message struct {
	raftpb.Message
	ReadCloser io.ReadCloser
	TotalSize  int64
	closeC     chan bool
}

// NewMessage returns a new Message from raftpb.Message.
//
// (etcd snap.NewMessage)
func NewMessage(msg raftpb.Message, rc io.ReadCloser, rcSize int64) *Message {
	return &Message{
		Message:    msg,
		ReadCloser: ioutil.NewExactReadCloser(rc, rcSize),
		TotalSize:  int64(msg.Size()) + rcSize,
		closeC:     make(chan bool, 1),
	}
}

// CloseNotify returns a channel that receives a single value
// when the message sent is finished. true indicates the sent
// is successful.
//
// (etcd snap.Message.CloseNotify)
func (m Message) CloseNotify() <-chan bool {
	return m.closeC
}

// CloseWithError closes with error.
//
// (etcd snap.Message.CloseWithError)
func (m Message) CloseWithError(err error) {
	if cerr := m.ReadCloser.Close(); cerr != nil {
		err = cerr
	}

	if err != nil {
		m.closeC <- false
		return
	}
	m.closeC <- true
}
