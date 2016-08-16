package rafthttp

import (
	"context"
	"sync"
	"time"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftsnap"
)

// (etcd rafthttp.peerGetter)
type peerGetter interface {
	Get(id types.ID) Peer
}

// Peer defines peer operations.
//
// (etcd rafthttp.Peer)
type Peer interface {
	sendMessageToPeer(msg raftpb.Message)
	sendSnapshotToPeer(msg raftsnap.Message)

	updatePeer(urls types.URLs)

	attachOutgoingConn(conn *outgoingConn)

	activeSince() time.Time

	stop()
}

// peer represents remote Raft node. Local Raft node uses peer to send messages to remote peers.
// stream is long-polling connection, always open to transfer messages.
// pipeline is a series of HTTP clients, and sends HTTP requests to remote peers.
// It is only used when the stream has not been established.
//
// (etcd rafthttp.peer)
type peer struct {
	peerID types.ID
	status *peerStatus
	r      Raft

	picker *urlPicker

	streamWriter   *streamWriter
	pipeline       *pipeline
	streamReader   *streamReader
	snapshotSender *snapshotSender

	sendc                     chan raftpb.Message
	incomingMessageCh         chan raftpb.Message // recv
	incomingProposalMessageCh chan raftpb.Message // propc
	stopc                     chan struct{}

	mu     sync.Mutex
	cancel context.CancelFunc
	paused bool
}

func (p *peer) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
}

func (p *peer) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
}

func startPeer(transport *Transport, peerID types.ID, peerURLs types.URLs) *peer {
	logger.Infof("starting peer %s", peerID)
	defer logger.Infof("started peer %s", peerID)

	status := newPeerStatus(peerID)
	r := transport.Raft
	picker := newURLPicker(peerURLs)

	pipeline := &pipeline{
		peerID:    peerID,
		status:    status,
		r:         r,
		picker:    picker,
		transport: transport,
		errc:      transport.errc,
	}
	pipeline.start()

	ctx, cancel := context.WithCancel(context.Background())
	p := &peer{
		peerID: peerID,
		status: status,
		r:      r,
		picker: picker,

		streamWriter: startStreamWriter(peerID, status, r),
		pipeline:     pipeline,

		snapshotSender: newSnapshotSender(transport, peerID, status, picker),

		sendc:                     make(chan raftpb.Message),
		incomingMessageCh:         make(chan raftpb.Message, receiveBufferN),
		incomingProposalMessageCh: make(chan raftpb.Message, maxPendingProposalN),
		stopc: make(chan struct{}),

		cancel: cancel,
	}

	go func() {
		for {
			select {
			case msg := <-p.incomingMessageCh: // case m := <-n.recvc:
				if err := r.Process(ctx, msg); err != nil {
					logger.Warningf("failed to process raft message (%v)", err)
				}
			case <-p.stopc:
				return
			}
		}
	}()
	go func() { // to avoid blocking incomingMessageCh
		for {
			select {
			case msg := <-p.incomingProposalMessageCh: // case m := <-propc:
				if err := r.Process(ctx, msg); err != nil {
					logger.Warningf("failed to process raft proposal message (%v)", err)
				}
			case <-p.stopc:
				return
			}
		}
	}()

	p.streamReader = &streamReader{
		peerID:                    peerID,
		status:                    status,
		picker:                    picker,
		transport:                 transport,
		incomingMessageCh:         p.incomingMessageCh,
		incomingProposalMessageCh: p.incomingProposalMessageCh,
	}
	p.streamReader.start()

	return p
}

// pick picks a channel to send the given message to.
//
// (etcd rafthttp.peer.pick)
func (p *peer) pick(msg raftpb.Message) (chan<- raftpb.Message, string) {
	// Considering MsgSnap may have a big size, e.g., 1G, and will block
	// stream for a long time, only use one of the N pipelines to send MsgSnap.
	if msg.Type == raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT {
		return p.pipeline.raftMessageChan, messageTypePipeline
	}

	if writec, ok := p.streamWriter.messageChanToSend(); ok {
		return writec, messageTypeMessage
	}

	return p.pipeline.raftMessageChan, messageTypePipeline
}

// / pick picks a chan for sending the given message. The picked chan and the picked chan
// // string name are returned.
// func (p *peer) pick(m raftpb.Message) (writec chan<- raftpb.Message, picked string) {

// 	if isMsgSnap(m) {
// 		return p.pipeline.msgc, pipelineMsg
// 	} else if writec, ok = p.msgAppV2Writer.writec(); ok && isMsgApp(m) {
// 		return writec, streamAppV2
// 	} else if writec, ok = p.writer.writec(); ok {
// 		return writec, streamMsg
// 	}
// 	return p.pipeline.msgc, pipelineMsg

// (etcd rafthttp.peer.send)
func (p *peer) sendMessageToPeer(msg raftpb.Message) {
	p.mu.Lock()
	paused := p.paused
	p.mu.Unlock()

	if paused {
		return
	}

	writec, name := p.pick(msg)
	select {
	case writec <- msg:
	default:
		p.r.ReportUnreachable(msg.To)
		if msg.Type == raftpb.MESSAGE_TYPE_LEADER_SNAPSHOT {
			p.r.ReportSnapshot(msg.To, raftpb.SNAPSHOT_STATUS_FAILED)
		}

		logger.Warningf("dropped %q(%s) from %s since sending buffer is full", msg.Type, name, types.ID(msg.From))
		if p.status.isActive() {
			logger.Warningf("%s network is bad/overloaded", p.peerID)
		}
	}
}

// (etcd rafthttp.peer.sendSnap)
func (p *peer) sendSnapshotToPeer(msg raftsnap.Message) {
	go p.snapshotSender.send(msg)
}

// (etcd rafthttp.peer.update)
func (p *peer) updatePeer(urls types.URLs) {
	p.picker.update(urls)
}

// (etcd rafthttp.peer.attachOutgoingConn)
func (p *peer) attachOutgoingConn(conn *outgoingConn) {
	if ok := p.streamWriter.attachOutgoingConn(conn); !ok {
		conn.Close()
	}
}

// (etcd rafthttp.peer.activeSince)
func (p *peer) activeSince() time.Time {
	return p.status.activeSince()
}

// (etcd rafthttp.peer.stop)
func (p *peer) stop() {
	logger.Infof("stopping peer %s", p.peerID)
	defer logger.Infof("stopped peer %s", p.peerID)

	close(p.stopc)
	p.cancel()
	p.streamWriter.stop()
	p.pipeline.stop()
	p.snapshotSender.stop()
	p.streamReader.stop()
}
