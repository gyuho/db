package rafthttp

import (
	"errors"
	"testing"
	"time"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
)

// (etcd rafthttp.TestStreamWriterAttachOutgoingConn)
func Test_streamWriter_attatchOutgoingConn(t *testing.T) {
	sw := startStreamWriter(types.ID(1), newPeerStatus(types.ID(1)), &fakeRaft{})
	if _, working := sw.messageChanToSend(); working {
		t.Fatalf("initial working status must be false, got %v", working)
	}

	var wfc *fakeWriterFlusherCloser
	for i := 0; i < 3; i++ {
		prev := wfc

		wfc = newFakeWriterFlusherCloser(nil)
		sw.attachOutgoingConn(&outgoingConn{Writer: wfc, Flusher: wfc, Closer: wfc})

		if prev != nil {
			select {
			case <-prev.closed:
			case <-time.After(time.Second):
				t.Fatalf("#%d: close of previous connection timed out", i)
			}
		}

		msgc, working := sw.messageChanToSend()
		if !working {
			t.Fatalf("#%d: working expected true, got %v", i, working)
		}
		msgc <- raftpb.Message{}

		select {
		case <-wfc.writec:
		case <-time.After(time.Second):
			t.Fatalf("#%d: failed to write to the underlying connection", i)
		}
		if _, working := sw.messageChanToSend(); !working {
			t.Fatalf("working expected true, got %v", working)
		}
	}

	sw.stop()

	if _, working := sw.messageChanToSend(); working {
		t.Fatalf("working expected false, got %v", working)
	}
	if !wfc.getClosed() {
		t.Fatalf("closed expected true, got %v", wfc.getClosed())
	}
}

// (etcd rafthttp.TestStreamWriterAttachBadOutgoingConn)
func Test_streamWriter_attatchOutgoingConn_bad(t *testing.T) {
	sw := startStreamWriter(types.ID(1), newPeerStatus(types.ID(1)), &fakeRaft{})
	defer sw.stop()

	wfc := newFakeWriterFlusherCloser(errors.New("test"))
	sw.attachOutgoingConn(&outgoingConn{Writer: wfc, Flusher: wfc, Closer: wfc})

	sw.msgc <- raftpb.Message{}
	select {
	case <-wfc.closed:
	case <-time.After(time.Second):
		t.Fatal("failed to close the underlying connection in time")
	}
	if _, working := sw.messageChanToSend(); working {
		t.Fatalf("working expected false, got %v", working)
	}
}
