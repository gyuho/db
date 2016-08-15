package rafthttp

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/version"
)

var (
	ErrMemberRemoved     = errors.New("rafthttp: the member has been permanently removed from the cluster") // (etcd rafthttp.errMemberRemoved)
	ErrMemberNotFound    = errors.New("rafthttp: member not found")                                         // (etcd rafthttp.errMemberNotFound)
	ErrStopped           = errors.New("rafthttp: stopped")                                                  // (etcd rafthttp.errStopped)
	ErrClusterIDMismatch = errors.New("rafthttp: cluster ID mismatch")                                      // (etcd rafthttp.errClusterIDMismatch)
)

const (
	// ConnWriteTimeout is I/O timeout for each connection, enough for recycling bad connections, otherwise minutes.
	//
	// (etcd rafthttp.ConnWriteTimeout)
	ConnWriteTimeout = 5 * time.Second

	// ConnReadTimeout is I/O timeout for each connection, enough for recycling bad connections, otherwise minutes.
	//
	// (etcd rafthttp.ConnReadTimeout)
	ConnReadTimeout = 5 * time.Second

	// snapResponseReadTimeout is timeout for reading snapshot response body.
	//
	// (etcd rafthttp.snapResponseReadTimeout)
	snapResponseReadTimeout = 5 * time.Second

	// maxConnReadByteN is the maximum number of bytes a single read can read out.
	//
	// 64KB should be big enough without causing throughput bottleneck,
	// and small enough to not cause read-timeout.
	//
	// (etcd rafthttp.connReadLimitByte)
	maxConnReadByteN = 64 * 1024

	// streamBufferN is the buffer size for stream.
	//
	// (etcd rafthttp.streamBufSize)
	streamBufferN = 4 * 1024

	// receiveBufferN is the buffer size for receiver.
	//
	// (etcd rafthttp.recvBufSize)
	receiveBufferN = 4 * 1024

	// pipelineBufferN is the buffer size of pipeline, to help hold temporary network latency.
	//
	// (etcd rafthttp.pipelineBufSize)
	pipelineBufferN = 64

	// connPerPipeline is the number of connections per pipeline.
	//
	// (etcd rafthttp.connPerPipeline)
	connPerPipeline = 4

	// maxPendingProposalN is the maximum number of proposals to hold
	// during one leader election process. Usually, election takes up to
	// 1-second.
	//
	// (etcd rafthttp.maxPendingProposals)
	maxPendingProposalN = 4 * 1024
)

var (
	PrefixRaft              = "/raft"                          // (etcd rafthttp.RaftPrefix)
	PrefixRaftProbing       = path.Join(PrefixRaft, "probing") // (etcd rafthttp.ProbingPrefix)
	PrefixRaftStream        = path.Join(PrefixRaft, "stream")  // (etcd rafthttp.RaftStreamPrefix)
	PrefixRaftStreamMessage = path.Join(PrefixRaft, "stream", "message")
	PrefixRaftSnapshot      = path.Join(PrefixRaft, "snapshot") // (etcd rafthttp.RaftSnapshotPrefix)
)

var (
	HeaderContentType     = "Content-Type"
	HeaderContentProtobuf = "application/protobuf"
	HeaderContentStream   = "application/octet-stream"

	HeaderFromID        = "X-rafthttp-From"           // X-Server-From
	HeaderToID          = "X-rafthttp-To"             // X-Raft-To
	HeaderClusterID     = "X-rafthttp-ClusterID"      // X-Etcd-Cluster-ID
	HeaderServerVersion = "X-rafthttp-Server-Version" // X-Server-Version
	HeaderPeerURLs      = "X-rafthttp-PeerURLs"       // X-PeerURLs
)

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

// emptyLeaderHeartbeat is a special heartbeat message without From, To fields.
//
// (etcd rafthttp.linkHeartbeatMessage)
var emptyLeaderHeartbeat = raftpb.Message{Type: raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT}

// (etcd rafthttp.isLinkHeartbeatMessage)
func isEmptyLeaderHeartbeat(msg raftpb.Message) bool {
	return msg.Type == raftpb.MESSAGE_TYPE_LEADER_HEARTBEAT && msg.From == 0 && msg.To == 0
}
