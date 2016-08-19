package rafthttp

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"

	dbioutil "github.com/gyuho/db/pkg/ioutil"
	"github.com/gyuho/db/pkg/netutil"
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

	status    *peerStatus
	picker    *urlPicker
	transport *Transport
	r         Raft

	stopc chan struct{}
	errc  chan error
}

// (etcd rafthttp.newSnapshotSender)
func newSnapshotSender(transport *Transport, to types.ID, status *peerStatus, picker *urlPicker) *snapshotSender {
	return &snapshotSender{
		from:      transport.Sender,
		to:        to,
		clusterID: transport.ClusterID,

		status:    status,
		picker:    picker,
		transport: transport,
		r:         transport.Raft,

		stopc: make(chan struct{}),
		errc:  transport.Errc,
	}
}

func createSnapBody(msg raftsnap.Message) io.ReadCloser {
	buf := new(bytes.Buffer)
	enc := raftpb.NewMessageBinaryEncoder(buf)
	if err := enc.Encode(&msg.Message); err != nil {
		logger.Panic(err)
	}
	return &dbioutil.ReaderAndCloser{
		Reader: io.MultiReader(buf, msg.ReadCloser),
		Closer: msg.ReadCloser,
	}
}

func (s *snapshotSender) stop() {
	close(s.stopc)
}

func (s *snapshotSender) post(req *http.Request) error {
	ctx, cancel := context.WithTimeout(context.TODO(), snapResponseReadTimeout)
	req = req.WithContext(ctx)

	errc := make(chan error)
	go func() {
		defer close(errc)

		resp, err := s.transport.pipelineRoundTripper.RoundTrip(req)
		if err != nil {
			errc <- err
			return
		}

		bts, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errc <- err
			return
		}

		if err = checkPostResponse(resp, bts, req, s.to); err != nil {
			errc <- err
			return
		}

		netutil.GracefulClose(resp)
	}()

	select {
	case <-s.stopc:
		cancel()
		return ErrStopped

	case err := <-errc:
		cancel()
		return err
	}
}

func (s *snapshotSender) send(msg raftsnap.Message) {
	rc := createSnapBody(msg)
	defer rc.Close()

	logger.Infof("starting snapshotSender.send to peer %s [index: %d]", types.ID(msg.To), msg.Snapshot.Metadata.Index)

	targetURL := s.picker.pick()
	req := createPostRequest(targetURL, PrefixRaftSnapshot, rc, HeaderContentStream, s.from, s.clusterID, s.transport.PeerURLs)

	err := s.post(req)
	defer msg.CloseWithError(err)

	if err != nil {
		logger.Warningf("failed to snapshotSender.send to peer %s [index: %d] (%v)", types.ID(msg.To), msg.Snapshot.Metadata.Index, err)

		if err == ErrMemberRemoved {
			sendError(err, s.errc)
		}

		s.status.deactivate(failureType{source: "snapshot post", action: "post", err: err})
		s.picker.unreachable(targetURL)
		s.r.ReportUnreachable(msg.To)
		s.r.ReportSnapshot(msg.To, raftpb.SNAPSHOT_STATUS_FAILED)
		return
	}

	s.status.activate()
	s.r.ReportSnapshot(msg.To, raftpb.SNAPSHOT_STATUS_FINISHED)

	logger.Infof("finished snapshotSender.send to peer %s [index: %d]", types.ID(msg.To), msg.Snapshot.Metadata.Index)
}
