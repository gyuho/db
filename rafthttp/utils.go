package rafthttp

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/version"
)

// emptyLeaderHeartbeat is a special heartbeat message without From, To fields.
//
// (etcd rafthttp.linkHeartbeatMessage)
var emptyLeaderHeartbeat = raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT}

// (etcd rafthttp.isLinkHeartbeatMessage)
func isEmptyLeaderHeartbeat(msg raftpb.Message) bool {
	return msg.Type == raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT && msg.From == 0 && msg.To == 0
}

// (etcd rafthttp.closeNotifier)
type closeNotifier struct{ donec chan struct{} }

// (etcd rafthttp.newCloseNotifier)
func newCloseNotifier() *closeNotifier {
	return &closeNotifier{donec: make(chan struct{})}
}

func (n *closeNotifier) Close() error {
	close(n.donec)
	return nil
}

func (n *closeNotifier) closeNotify() <-chan struct{} { return n.donec }

// (etcd rafthttp.writerToResponse)
type writerToResponse interface {
	WriteTo(rw http.ResponseWriter)
}

// (etcd rafthttp.setPeerURLsHeader)
func setHeaderPeerURLs(req *http.Request, peerURLs types.URLs) {
	if peerURLs == nil {
		return
	}
	ps := make([]string, peerURLs.Len())
	for i := range peerURLs {
		ps[i] = peerURLs[i].String()
	}
	req.Header.Set(HeaderPeerURLs, strings.Join(ps, ","))
}

// (etcd rafthttp.createPostRequest)
func createPostRequest(target url.URL, path string, rd io.Reader, contentType string, from, clusterID types.ID, peerURLs types.URLs) *http.Request {
	uu := target
	uu.Path = path

	req, err := http.NewRequest("POST", uu.String(), rd)
	if err != nil {
		logger.Panic(err)
	}

	req.Header.Set(HeaderContentType, contentType)
	req.Header.Set(HeaderFromID, from.String())
	req.Header.Set(HeaderClusterID, clusterID.String())
	req.Header.Set(HeaderServerVersion, version.ServerVersion)

	setHeaderPeerURLs(req, peerURLs)

	return req
}

// (etcd rafthttp.checkPostResponse)
func checkPostResponse(resp *http.Response, body []byte, req *http.Request, peerID types.ID) error {
	switch resp.StatusCode {
	case http.StatusPreconditionFailed:
		switch strings.TrimSuffix(string(body), "\n") {
		case ErrClusterIDMismatch.Error():
			logger.Errorf("request was ignored (%v, remote[%s]=%s, local=%s)", ErrClusterIDMismatch, peerID, resp.Header.Get(HeaderClusterID), req.Header.Get(HeaderClusterID))
			return ErrClusterIDMismatch
		default:
			return fmt.Errorf("unhandled error %q", body)
		}

	case http.StatusForbidden:
		return ErrMemberRemoved

	case http.StatusNoContent:
		return nil

	default:
		return fmt.Errorf("unexpected http status %s while posting to %q", http.StatusText(resp.StatusCode), req.URL.String())
	}
}

// (etcd rafthttp.reportCriticalError)
func sendError(err error, errc chan<- error) {
	select {
	case errc <- err:
	default:
	}
}
