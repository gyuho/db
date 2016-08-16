package rafthttp

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	dbioutil "github.com/gyuho/db/pkg/ioutil"
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
)

// (etcd rafthttp.pipelineHandler)
type pipelineHandler struct {
	tr        Transporter
	r         Raft
	clusterID types.ID
}

// (etcd rafthttp.newPipelineHandler)
func newPipelineHandler(tr Transporter, r Raft, clusterID types.ID) http.Handler {
	return &pipelineHandler{
		tr:        tr,
		r:         r,
		clusterID: clusterID,
	}
}

func (hd *pipelineHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		rw.Header().Set("Allow", "POST")
		http.Error(rw, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	rw.Header().Set(HeaderClusterID, hd.clusterID.String())

	if err := checkClusterCompatibilityFromHeader(req.Header, hd.clusterID); err != nil {
		http.Error(rw, err.Error(), http.StatusPreconditionFailed)
		return
	}

	if from, err := types.IDFromString(req.Header.Get(HeaderFromID)); err != nil { // ???
		if urls := req.Header.Get(HeaderPeerURLs); urls != "" {
			hd.tr.AddPeerRemote(from, strings.Split(urls, ","))
		}
	}

	limitedRd := dbioutil.NewLimitedBufferReader(req.Body, maxConnReadByteN)
	bts, err := ioutil.ReadAll(limitedRd)
	if err != nil {
		errMsg := fmt.Sprintf("failed to read raft message (%v)", err)
		logger.Errorln(errMsg)
		http.Error(rw, errMsg, http.StatusBadRequest)
		return
	}

	var msg raftpb.Message
	if err := msg.Unmarshal(bts); err != nil {
		errMsg := fmt.Sprintf("failed to unmarshal raft message (%v)", err)
		logger.Errorln(errMsg)
		http.Error(rw, errMsg, http.StatusBadRequest)
		return
	}

	if err := hd.r.Process(context.TODO(), msg); err != nil {
		switch v := err.(type) {
		case writerToResponse:
			v.WriteTo(rw)

		default:
			errMsg := fmt.Sprintf("failed to process raft message (%v)", err)
			logger.Warningln(errMsg)
			http.Error(rw, errMsg, http.StatusInternalServerError)
			rw.(http.Flusher).Flush()

			// disconnect the http stream
			panic(err)
		}

		return
	}

	rw.WriteHeader(http.StatusNoContent)
}
