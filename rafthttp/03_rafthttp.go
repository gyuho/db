package rafthttp

import (
	"errors"
	"path"
	"time"
)

var (
	ErrMemberRemoved         = errors.New("rafthttp: the member has been permanently removed from the cluster") // (etcd rafthttp.errMemberRemoved)
	ErrMemberNotFound        = errors.New("rafthttp: member not found")                                         // (etcd rafthttp.errMemberNotFound)
	ErrStopped               = errors.New("rafthttp: stopped")                                                  // (etcd rafthttp.errStopped)
	ErrUnsupportedStreamType = errors.New("rafthttp: unsupported stream type")                                  // (etcd rafthttp.errUnsupportedStreamType)
	ErrIncompatibleVersion   = errors.New("rafthttp: incompatible version")                                     // (etcd rafthttp.errIncompatibleVersion)
	ErrClusterIDMismatch     = errors.New("rafthttp: cluster ID mismatch")                                      // (etcd rafthttp.errClusterIDMismatch)
)

var (
	PrefixRaft         = "/raft"                           // (etcd rafthttp.RaftPrefix)
	PrefixRaftProbing  = path.Join(PrefixRaft, "probing")  // (etcd rafthttp.ProbingPrefix)
	PrefixRaftStream   = path.Join(PrefixRaft, "stream")   // (etcd rafthttp.RaftStreamPrefix)
	PrefixRaftSnapshot = path.Join(PrefixRaft, "snapshot") // (etcd rafthttp.RaftSnapshotPrefix)
)

var (
	HeaderVersion   = "X-rafthttp-Version"   // X-Server-Version
	HeaderClusterID = "X-rafthttp-ClusterID" // X-Etcd-Cluster-ID
	HeaderPeerURLs  = "X-rafthttp-PeerURLs"  // X-PeerURLs
	HeaderFromID    = "X-rafthttp-From"      // X-Server-From
	HeaderToID      = "X-rafthttp-To"        // X-Raft-To
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
