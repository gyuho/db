package rafthttp

import (
	"errors"
	"net/http"
	"path"
	"time"

	"github.com/gyuho/db/pkg/types"
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
	messageTypeMessage  = "message"  // (etcd rafthttp.streamMsg)
	messageTypePipeline = "pipeline" // (etcd rafthttp.pipelineMsg)

	PrefixRaft              = "/raft"                          // (etcd rafthttp.RaftPrefix)
	PrefixRaftProbing       = path.Join(PrefixRaft, "probing") // (etcd rafthttp.ProbingPrefix)
	PrefixRaftStream        = path.Join(PrefixRaft, "stream")  // (etcd rafthttp.RaftStreamPrefix)
	PrefixRaftStreamMessage = path.Join(PrefixRaft, "stream", messageTypeMessage)
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

// (etcd rafthttp.checkClusterCompatibilityFromHeader)
func checkClusterCompatibilityFromHeader(header http.Header, clusterID types.ID) error {
	if gclusterID := header.Get(HeaderClusterID); gclusterID != clusterID.String() {
		logger.Errorf("request cluster ID mismatch (got %s want %s)", gclusterID, clusterID)
		return ErrClusterIDMismatch
	}
	return nil
}
