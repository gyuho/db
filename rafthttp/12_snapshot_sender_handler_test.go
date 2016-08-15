package rafthttp

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gyuho/db/pkg/types"
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
func Test_snapshotSender(t *testing.T) {

}
