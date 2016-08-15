package rafthttp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftsnap"
)

// (etcd rafthttp.snapshotHandler)
type snapshotSenderHandler struct {
	tr          Transporter
	r           Raft
	snapshotter *raftsnap.Snapshotter
	clusterID   types.ID
}

// (etcd rafthttp.newSnapshotHandler)
func newSnapshotSenderHandler(tr Transporter, r Raft, snapshotter *raftsnap.Snapshotter, clusterID types.ID) http.Handler {
	return &snapshotSenderHandler{
		tr:          tr,
		r:           r,
		snapshotter: snapshotter,
		clusterID:   clusterID,
	}
}

func (hd *snapshotSenderHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		rw.Header().Set("Allow", "POST")
		http.Error(rw, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	rw.Header().Set(HeaderClusterID, hd.clusterID.String())

	if from, err := types.IDFromString(req.Header.Get(HeaderFromID)); err != nil { // ???
		if urls := req.Header.Get(HeaderPeerURLs); urls != "" {
			hd.tr.AddPeerRemote(from, strings.Split(urls, ","))
		}
	}

	dec := raftpb.NewMessageBinaryDecoder(req.Body)
	msg, err := dec.Decode()
	if err != nil {
		errMsg := fmt.Sprintf("failed to decode raft message (%v)", err)
		logger.Errorln(errMsg)
		http.Error(rw, errMsg, http.StatusBadRequest)
		return
	}

	if msg.Type != raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT {
		errMsg := fmt.Sprintf("unexpected raft message type %q", msg.Type)
		logger.Errorln(errMsg)
		http.Error(rw, errMsg, http.StatusBadRequest)
		return
	}

	logger.Infof("receiving snapshot from peer %s [index: %d]", types.ID(msg.From), msg.Snapshot.Metadata.Index)
	_, err = hd.snapshotter.SaveDB(req.Body, msg.Snapshot.Metadata.Index)
	if err != nil {
		errMsg := fmt.Sprintf("failed to save snapshot (%v)", err)
		logger.Errorln(errMsg)
		http.Error(rw, errMsg, http.StatusBadRequest)
		return
	}
	logger.Infof("received snapshot from peer %s [index: %d]", types.ID(msg.From), msg.Snapshot.Metadata.Index)

	if err := hd.r.Process(context.TODO(), msg); err != nil {
		switch v := err.(type) {
		case writerToResponse:
			v.WriteTo(rw)

		default:
			errMsg := fmt.Sprintf("failed to process raft message (%v)", err)
			logger.Warningln(errMsg)
			http.Error(rw, errMsg, http.StatusBadRequest)
			rw.(http.Flusher).Flush()

			// disconnect the http stream
			panic(err)
		}

		return
	}

	rw.WriteHeader(http.StatusNoContent)
}
