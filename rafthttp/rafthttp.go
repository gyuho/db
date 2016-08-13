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
	RaftPrefix         = "/raft"
	ProbingPrefix      = path.Join(RaftPrefix, "probing")
	RaftStreamPrefix   = path.Join(RaftPrefix, "stream")
	RaftSnapshotPrefix = path.Join(RaftPrefix, "snapshot")
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

	// MaxConnReadByteN is the maximum number of bytes a single read can read out.
	//
	// 64KB should be big enough without causing throughput bottleneck,
	// and small enough to not cause read-timeout.
	//
	// (etcd rafthttp.connReadLimitByte)
	MaxConnReadByteN = 64 * 1024

	// MaxReceiveBufferN is the maximum buffer size for receiver.
	//
	// (etcd rafthttp.recvBufSize)
	MaxReceiveBufferN = 4 * 1024

	// MaxPendingProposalN is the maximum number of proposals to hold
	// during one leader election process. Usually, election takes up to
	// 1-second.
	//
	// (etcd rafthttp.maxPendingProposals)
	MaxPendingProposalN = 4 * 1024
)
