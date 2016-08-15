package rafthttp

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gyuho/db/pkg/netutil"
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/version"
)

// streamReader reads messages from remote peers.
//
// (etcd rafthttp.streamReader)
type streamReader struct {
	peerID types.ID
	status *peerStatus

	picker *urlPicker
	pt     *PeerTransport

	recvc chan<- raftpb.Message
	propc chan<- raftpb.Message
	stopc chan struct{}
	donec chan struct{}
	errc  chan<- error

	mu     sync.Mutex
	paused bool
	cancel func()
	closer io.Closer
}

func (sr *streamReader) pause() {
	sr.mu.Lock()
	sr.paused = true
	sr.mu.Unlock()
}

func (sr *streamReader) resume() {
	sr.mu.Lock()
	sr.paused = false
	sr.mu.Unlock()
}

func (sr *streamReader) close() {
	if sr.closer != nil {
		sr.closer.Close()
	}
	sr.closer = nil
}

func (sr *streamReader) stop() {
	close(sr.stopc)

	sr.mu.Lock()
	if sr.cancel != nil {
		sr.cancel()
	}
	sr.close()
	sr.mu.Unlock()

	<-sr.donec
}

func (sr *streamReader) dial() (io.ReadCloser, error) {
	targetURL := sr.picker.pick()
	uu := targetURL
	uu.Path = path.Join(PrefixRaftStreamMessage, sr.pt.From.String())

	req, err := http.NewRequest("GET", uu.String(), nil)
	if err != nil {
		sr.picker.unreachable(targetURL)
		return nil, fmt.Errorf("failed to request to %v (%v)", targetURL, err)
	}

	// req.Header.Set(HeaderContentType, contentType)
	req.Header.Set(HeaderFromID, sr.pt.From.String())
	req.Header.Set(HeaderToID, sr.peerID.String())
	req.Header.Set(HeaderClusterID, sr.pt.ClusterID.String())
	req.Header.Set(HeaderServerVersion, version.ServerVersion)

	setHeaderPeerURLs(req, sr.pt.PeerURLs)

	sr.mu.Lock()
	select {
	case <-sr.stopc:
		sr.mu.Unlock()
		return nil, fmt.Errorf("streamReader is stopped")
	default:
	}
	ctx, cancel := context.WithCancel(context.TODO())
	req = req.WithContext(ctx)
	sr.cancel = cancel
	sr.mu.Unlock()

	resp, err := sr.pt.streamRoundTripper.RoundTrip(req)
	if err != nil {
		sr.picker.unreachable(targetURL)
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return resp.Body, nil

	case http.StatusGone:
		netutil.GracefulClose(resp)
		sr.picker.unreachable(targetURL)
		sendError(ErrMemberRemoved, sr.errc)
		return nil, ErrMemberRemoved

	case http.StatusNotFound:
		netutil.GracefulClose(resp)
		sr.picker.unreachable(targetURL)
		return nil, fmt.Errorf("peer %s failed to find local member %s", sr.peerID, sr.pt.From)

	case http.StatusPreconditionFailed:
		bts, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			sr.picker.unreachable(targetURL)
			return nil, err
		}

		netutil.GracefulClose(resp)
		sr.picker.unreachable(targetURL)

		switch strings.TrimSuffix(string(bts), "\n") {
		case ErrClusterIDMismatch.Error():
			logger.Errorf("request was ignored (%v, remote[%s]=%s, local=%s)", ErrClusterIDMismatch, sr.peerID, resp.Header.Get(HeaderClusterID), req.Header.Get(HeaderClusterID))
			return nil, ErrClusterIDMismatch

		default:
			return nil, fmt.Errorf("unhandled error %q", bts)
		}

	default:
		netutil.GracefulClose(resp)
		sr.picker.unreachable(targetURL)
		return nil, fmt.Errorf("unhandled http status %d", resp.StatusCode)
	}
}

func (sr *streamReader) decodeLoop(rc io.ReadCloser) error {
	sr.mu.Lock()
	sr.closer = rc
	sr.mu.Unlock()

	dec := raftpb.NewMessageBinaryDecoder(rc)

	for {
		msg, err := dec.Decode()
		if err != nil {
			sr.mu.Lock()
			sr.close()
			sr.mu.Unlock()
			return err
		}

		sr.mu.Lock()
		paused := sr.paused
		sr.mu.Unlock()
		if paused {
			continue
		}

		if isEmptyLeaderHeartbeat(msg) {
			continue
		}

		recvc := sr.recvc
		if msg.Type == raftpb.MESSAGE_TYPE_PROPOSAL_TO_LEADER {
			recvc = sr.propc
		}

		select {
		case recvc <- msg:
		default:
			logger.Warningf("dropped %q from %s since receiving buffer is full", msg.Type, types.ID(msg.From))
			if sr.status.isActive() {
				logger.Warningf("%s network is overloaded", sr.peerID)
			}
		}
	}
}

func (sr *streamReader) start() {
	sr.stopc = make(chan struct{})
	sr.donec = make(chan struct{})
	if sr.errc == nil {
		sr.errc = sr.pt.errc
	}

	logger.Infof("started streamReader to peer %s", sr.peerID)
	go sr.run()
}

func (sr *streamReader) run() {
	for {
		rc, err := sr.dial()
		if err != nil {
			sr.status.deactivate(failureType{source: "stream dial", action: "dial", err: err})
		} else {
			sr.status.activate()
			logger.Infof("established streamReader to peer %s", sr.peerID)

			err = sr.decodeLoop(rc)
			logger.Warningf("lost streamReader connection to peer %s", sr.peerID)
			switch {
			case err == io.EOF, netutil.IsClosedConnectionError(err):
				logger.Warningf("connection lost; remote closed")
			default:
				sr.status.deactivate(failureType{source: "stream decodeLoop", action: "decode", err: err})
			}
		}

		select {
		case <-time.After(100 * time.Millisecond):
		case <-sr.stopc:
			close(sr.donec)
			logger.Infof("stopped streamReader to peer %s", sr.peerID)
			return
		}
	}
}
