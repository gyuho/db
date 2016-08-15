package rafthttp

import (
	"bytes"
	"io"

	"github.com/gyuho/db/pkg/ioutil"
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftsnap"
)

// snapshotSender contains PeerTransport to dial to remote endpoints.
// It sends snapshot to peers.
//
// (etcd rafthttp.snapshotSender)
type snapshotSender struct {
	from      types.ID
	to        types.ID
	clusterID types.ID

	status *peerStatus

	picker    *urlPicker
	transport *Transport

	r Raft

	stopc chan struct{}
	errc  chan error
}

// (etcd rafthttp.newSnapshotSender)
func newSnapshotSender(transport *Transport, to types.ID, status *peerStatus, picker *urlPicker) *snapshotSender {
	return &snapshotSender{
		from:      transport.From,
		to:        to,
		clusterID: transport.ClusterID,

		status: status,

		picker:    picker,
		transport: transport,

		r: transport.Raft,

		stopc: make(chan struct{}),
		errc:  transport.errc,
	}
}

func (s *snapshotSender) stop() {
	close(s.stopc)
}

func createSnapBody(msg raftsnap.Message) io.ReadCloser {
	buf := new(bytes.Buffer)
	enc := raftpb.NewMessageBinaryEncoder(buf)
	if err := enc.Encode(&msg.RaftMessage); err != nil {
		logger.Panic(err)
	}
	return &ioutil.ReaderAndCloser{
		Reader: io.MultiReader(buf, msg.ReadCloser),
		Closer: msg.ReadCloser,
	}
}
