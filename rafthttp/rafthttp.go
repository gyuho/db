package rafthttp

import (
	"errors"
	"time"
)

var (
	ErrMemberRemoved  = errors.New("rafthttp: the member has been permanently removed from the cluster") // (etcd rafthttp.errMemberRemoved)
	ErrMemberNotFound = errors.New("rafthttp: member not found")                                         // (etcd rafthttp.errMemberNotFound)
	ErrStopped        = errors.New("rafthttp: stopped")                                                  // (etcd rafthttp.errStopped)
)

const (
	// ConnWriteTimeout is I/O timeout for each connection, enough for recycling bad connections, otherwise minutes.
	ConnWriteTimeout = 5 * time.Second

	// ConnReadTimeout is I/O timeout for each connection, enough for recycling bad connections, otherwise minutes.
	ConnReadTimeout = 5 * time.Second

	receiveBufferN = 1024 * 4 // (etcd rafthttp.recvBufSize)

	// maxPendingProposalN is the maximum number of proposals to hold
	// during one leader election process. Usually, election takes up to
	// 1-second.
	//
	// (etcd rafthttp.maxPendingProposals)
	maxPendingProposalN = 1024 * 4
)
