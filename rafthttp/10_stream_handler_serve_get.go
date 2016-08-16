package rafthttp

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/version"
)

// (etcd rafthttp.streamHandler)
type streamHandler struct {
	tr         Transporter
	r          Raft
	peerGetter peerGetter

	id        types.ID
	clusterID types.ID
}

// (etcd rafthttp.newStreamHandler)
func newStreamHandler(tr Transporter, r Raft, pg peerGetter, id, clusterID types.ID) http.Handler {
	return &streamHandler{
		tr:         tr,
		r:          r,
		peerGetter: pg,

		id:        id,
		clusterID: clusterID,
	}
}

func (hd *streamHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		rw.Header().Set("Allow", "GET")
		http.Error(rw, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	rw.Header().Set(HeaderServerVersion, version.ServerVersion)
	rw.Header().Set(HeaderClusterID, hd.clusterID.String())

	if err := checkClusterCompatibilityFromHeader(req.Header, hd.clusterID); err != nil {
		http.Error(rw, err.Error(), http.StatusPreconditionFailed)
		return
	}

	switch path.Dir(req.URL.Path) {
	case PrefixRaftStreamMessage:
	default:
		errMsg := fmt.Sprintf("unexpected stream request path %q", req.URL.Path)
		logger.Warningln(errMsg)
		http.Error(rw, errMsg, http.StatusNotFound)
		return
	}

	if from, err := types.IDFromString(req.Header.Get(HeaderFromID)); err != nil { // ???
		if urls := req.Header.Get(HeaderPeerURLs); urls != "" {
			hd.tr.AddPeerRemote(from, strings.Split(urls, ","))
		}
	}

	fromStr := path.Base(req.URL.Path)
	from, err := types.IDFromString(fromStr)
	if err != nil {
		errMsg := fmt.Sprintf("failed to parse from-ID %s (%v)", fromStr, err)
		logger.Errorln(errMsg)
		http.Error(rw, errMsg, http.StatusNotFound)
		return
	}

	if hd.r.IsIDRemoved(uint64(from)) {
		errMsg := fmt.Sprintf("rejected stream from peer %s since it was removed", from)
		logger.Warningln(errMsg)
		http.Error(rw, errMsg, http.StatusGone)
		return
	}

	pg := hd.peerGetter.Get(from) // ???
	if pg == nil {
		if urls := req.Header.Get(HeaderPeerURLs); urls != "" {
			hd.tr.AddPeerRemote(from, strings.Split(urls, ","))
		}
		errMsg := fmt.Sprintf("%s failed to find member [fromID: %s, clusterID: %s]", hd.id, from, hd.clusterID)
		logger.Errorln(errMsg)
		http.Error(rw, errMsg, http.StatusNotFound)
		return
	}

	if hdid, hid := hd.id.String(), req.Header.Get(HeaderToID); hdid != hid {
		errMsg := fmt.Sprintf("streaming request ignored because of to-ID mismatch (%s != %s)", hdid, hid)
		logger.Errorln(errMsg)
		http.Error(rw, errMsg, http.StatusPreconditionFailed)
		return
	}

	rw.WriteHeader(http.StatusOK)
	rw.(http.Flusher).Flush()

	cn := newCloseNotifier()
	conn := &outgoingConn{
		Writer:  rw,
		Flusher: rw.(http.Flusher),
		Closer:  cn,
	}
	pg.attachOutgoingConn(conn)
	<-cn.closeNotify()
}
