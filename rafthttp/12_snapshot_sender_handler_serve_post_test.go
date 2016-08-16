package rafthttp

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftsnap"
)

// (etcd rafthttp.testSnapshotSend)
func testSnapshotSend(t *testing.T, sm *raftsnap.Message) (bool, []os.FileInfo) {
	d, err := ioutil.TempDir(os.TempDir(), "snapdir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	r := &fakeRaft{}
	transport := &Transport{ClusterID: types.ID(1), Raft: r, pipelineRoundTripper: &http.Transport{}}

	ch := make(chan struct{}, 1)
	hd := &syncHandler{
		h:  newSnapshotSenderHandler(transport, r, raftsnap.New(d), types.ID(1)),
		ch: ch,
	}

	srv := httptest.NewServer(hd)
	defer srv.Close()

	sender := newSnapshotSender(transport, types.ID(1), newPeerStatus(types.ID(1)), newURLPicker(types.MustNewURLs([]string{srv.URL})))
	defer sender.stop()

	sender.send(*sm)

	sent := false
	select {
	case <-time.After(time.Second):
		t.Fatalf("timed out sending snapshot")
	case sent = <-sm.CloseNotify():
	}

	// wait for handler to finish accepting snapshot
	<-ch

	files, err := ioutil.ReadDir(d)
	if err != nil {
		t.Fatal(err)
	}

	return sent, files
}

// (etcd rafthttp.TestSnapshotSend)
func Test_snapshotSender_snapshotSenderHandler(t *testing.T) {
	tests := []struct {
		msg  raftpb.Message
		rc   io.ReadCloser
		size int64

		wsent  bool
		wfiles int
	}{
		{ // sent, received with no error
			msg:  raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT, To: 1},
			rc:   stringReaderCloser{strings.NewReader("testdata")},
			size: 8, // size of 'testdata'

			wsent:  true,
			wfiles: 1,
		},

		{ // send, and error
			msg:  raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT, To: 1},
			rc:   &errorReaderCloser{errors.New("snapshot error")},
			size: 1,

			wsent:  false,
			wfiles: 0,
		},

		{ // send less than given snapshot size
			msg:  raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT, To: 1},
			rc:   stringReaderCloser{strings.NewReader("testdata")},
			size: 100000,

			wsent:  false,
			wfiles: 0,
		},

		{ // send less than actual snapshot length
			msg:  raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT, To: 1},
			rc:   stringReaderCloser{strings.NewReader("testdata")},
			size: 1,

			wsent:  false,
			wfiles: 0,
		},
	}

	for i, tt := range tests {
		sent, files := testSnapshotSend(t, raftsnap.NewMessage(tt.msg, tt.rc, tt.size))
		if sent != tt.wsent {
			t.Fatalf("#%d: wsent expected %v, got %v", i, tt.wsent, sent)
		}
		if len(files) != tt.wfiles {
			t.Fatalf("#%d: wfiles expected %d, got %d", i, tt.wfiles, len(files))
		}
	}
}
