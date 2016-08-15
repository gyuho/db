package rafthttp

import (
	"bytes"
	"context"
	"io/ioutil"
	"sync"

	"github.com/gyuho/db/pkg/scheduleutil"
	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
)

// pipeline contains Transport.
// pipeline handles a series of HTTP clients, and sends thoses to remote peers.
// It is only used when the stream has not been established.
//
// (etcd rafthttp.pipeline)
type pipeline struct {
	peerID types.ID
	status *peerStatus

	r Raft

	picker    *urlPicker
	transport *Transport
	errc      chan error

	raftMessageChan chan raftpb.Message
	stopc           chan struct{}

	connWg sync.WaitGroup
}

func (p *pipeline) start() {
	p.raftMessageChan = make(chan raftpb.Message, pipelineBufferN)
	p.stopc = make(chan struct{})
	p.connWg.Add(connPerPipeline)

	for i := 0; i < connPerPipeline; i++ {
		go p.handle()
	}

	logger.Infof("started pipeline to peer %s", p.peerID)
}

func (p *pipeline) stop() {
	close(p.stopc)
	p.connWg.Wait()
	logger.Infof("stopped pipeline to peer %s", p.peerID)
}

func (p *pipeline) post(data []byte) error {
	targetURL := p.picker.pick()
	req := createPostRequest(targetURL, PrefixRaft, bytes.NewBuffer(data), HeaderContentProtobuf, p.transport.From, p.transport.ClusterID, p.transport.PeerURLs)

	ctx, cancel := context.WithCancel(context.TODO())
	req = req.WithContext(ctx)
	donec := make(chan struct{})
	go func() {
		select {
		case <-donec:
		case <-p.stopc:
			scheduleutil.WaitGoSchedule()
			cancel()
		}
	}()

	resp, err := p.transport.pipelineRoundTripper.RoundTrip(req)
	close(donec)
	if err != nil {
		p.picker.unreachable(targetURL)
		return err
	}

	bts, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		p.picker.unreachable(targetURL)
		return err
	}
	resp.Body.Close()

	err = checkPostResponse(resp, bts, req, p.peerID)
	if err != nil {
		p.picker.unreachable(targetURL)
		if err == ErrMemberRemoved {
			sendError(err, p.errc)
		}
		return err
	}

	return nil
}

func (p *pipeline) handle() {
	defer p.connWg.Done()

	for {
		select {
		case msg := <-p.raftMessageChan:
			bts, err := msg.Marshal()
			if err != nil {
				p.status.deactivate(failureType{source: "pipeline message", action: "write", err: err})
				p.r.ReportUnreachable(msg.To)
				if msg.Type == raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT {
					p.r.ReportSnapshot(msg.To, raftpb.SNAPSHOT_STATUS_FAILED)
				}
				continue
			}

			if err := p.post(bts); err != nil {
				p.status.deactivate(failureType{source: "pipeline message", action: "post", err: err})
				p.r.ReportUnreachable(msg.To)
				if msg.Type == raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT {
					p.r.ReportSnapshot(msg.To, raftpb.SNAPSHOT_STATUS_FAILED)
				}
				continue
			}

			p.status.activate()
			if msg.Type == raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT {
				p.r.ReportSnapshot(msg.To, raftpb.SNAPSHOT_STATUS_FINISHED)
			}

		case <-p.stopc:
			return
		}
	}
}
