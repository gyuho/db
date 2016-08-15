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

// peerPipeline contains PeerTransport.
// peerPipeline handles a series of HTTP clients, and sends thoses to remote peers.
// It is only used when the stream has not been established.
//
// (etcd rafthttp.pipeline)
type peerPipeline struct {
	peerID types.ID
	status *peerStatus

	r Raft

	picker *urlPicker
	pt     *PeerTransport
	errc   chan error

	raftMessageChan chan raftpb.Message
	stopc           chan struct{}

	connWg sync.WaitGroup
}

func (p *peerPipeline) start() {
	p.raftMessageChan = make(chan raftpb.Message, peerPipelineBufferN)
	p.stopc = make(chan struct{})
	p.connWg.Add(connPerPipeline)

	for i := 0; i < connPerPipeline; i++ {
		go p.handle()
	}

	logger.Infof("started peerPipeline to peer %s", p.peerID)
}

func (p *peerPipeline) stop() {
	close(p.stopc)
	p.connWg.Wait()
	logger.Infof("stopped peerPipeline to peer %s", p.peerID)
}

func (p *peerPipeline) post(data []byte) error {
	targetURL := p.picker.pick()
	req := createPostRequest(targetURL, PrefixRaft, bytes.NewBuffer(data), HeaderContentProtobuf, p.pt.From, p.pt.ClusterID, p.pt.PeerURLs)

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

	resp, err := p.pt.peerPipelineRoundTripper.RoundTrip(req)
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

func (p *peerPipeline) handle() {
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
