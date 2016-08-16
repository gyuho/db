package rafthttp

import (
	"errors"
	"testing"

	"github.com/gyuho/db/pkg/scheduleutil"
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

		wfc = &fakeWriterFlusherCloser{}
		sw.attachOutgoingConn(&outgoingConn{Writer: wfc, Flusher: wfc, Closer: wfc})

		for j := 0; j < 3; j++ {
			scheduleutil.WaitSchedule()

			// previous connection should be closed; if not wait
			if prev != nil && !prev.getClosed() {
				continue
			}
			if _, working := sw.messageChanToSend(); !working {
				continue
			}
		}

		// previous connection must be closed
		if prev != nil && !prev.getClosed() {
			t.Fatalf("previous outgoingConn must be closed, got %v", prev.getClosed())
		}
		if _, working := sw.messageChanToSend(); !working {
			t.Fatalf("working expected true, got %v", working)
		}

		sw.raftMessageChan <- raftpb.Message{}

		scheduleutil.WaitSchedule()

		if _, working := sw.messageChanToSend(); !working {
			t.Fatalf("working expected true, got %v", working)
		}
		if wfc.getWritten() == 0 {
			t.Fatalf("should have written, got %d", wfc.getWritten())
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

	wfc := &fakeWriterFlusherCloser{err: errors.New("test")}
	sw.attachOutgoingConn(&outgoingConn{Writer: wfc, Flusher: wfc, Closer: wfc})

	sw.raftMessageChan <- raftpb.Message{}

	scheduleutil.WaitSchedule()

	if _, working := sw.messageChanToSend(); working {
		t.Fatalf("working expected false, got %v", working)
	}
	if !wfc.getClosed() {
		t.Fatalf("closed expected true, got %v", wfc.getClosed())
	}
}
