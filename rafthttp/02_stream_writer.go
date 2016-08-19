package rafthttp

import (
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
)

// (etcd rafthttp.outgoingConn)
type outgoingConn struct {
	io.Writer
	http.Flusher
	io.Closer
}

// streamWriter writes messages to the attached outgoingConn.
//
// (etcd rafthttp.streamWriter)
type streamWriter struct {
	peerID types.ID
	status *peerStatus

	r Raft

	connc chan *outgoingConn
	msgc  chan raftpb.Message

	stopc chan struct{}
	donec chan struct{}

	mu      sync.Mutex
	closer  io.Closer
	working bool
}

// (etcd rafthttp.startStreamWriter)
func startStreamWriter(peerID types.ID, status *peerStatus, r Raft) *streamWriter {
	wr := &streamWriter{
		peerID: peerID,
		status: status,
		r:      r,

		connc: make(chan *outgoingConn),
		msgc:  make(chan raftpb.Message, streamBufferN),

		stopc: make(chan struct{}),
		donec: make(chan struct{}),
	}

	go wr.run()

	return wr
}

// closeWriter closes streamWriter and returns true if closed successfully.
//
// (etcd rafthttp.streamWriter.close)
func (sw *streamWriter) closeWriter() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.unsafeCloseWriter()
}

// (etcd rafthttp.streamWriter.closeUnlocked)
func (sw *streamWriter) unsafeCloseWriter() bool {
	logger.Warningf("closing streamWriter to peer %s", sw.peerID)
	if !sw.working {
		logger.Infof("streamWriter to peer %s is already not working", sw.peerID)
		return false
	}

	sw.closer.Close()

	if len(sw.msgc) > 0 {
		// messages were sent to channel, but we just closed it
		sw.r.ReportUnreachable(uint64(sw.peerID))
	}

	sw.msgc = make(chan raftpb.Message, streamBufferN)
	sw.working = false

	logger.Warningf("closed streamWriter to peer %s", sw.peerID)
	return true
}

// (etcd rafthttp.streamWriter.stop)
func (sw *streamWriter) stop() {
	logger.Infof("stopping streamWriter to peer %s", sw.peerID)
	close(sw.stopc)
	<-sw.donec
	logger.Infof("stopped streamWriter to peer %s", sw.peerID)
}

// (etcd rafthttp.streamWriter.run)
func (sw *streamWriter) run() {
	var (
		messageBinaryEncoder *raftpb.MessageBinaryEncoder
		httpFlusher          http.Flusher

		batchedN int

		msgc   chan raftpb.Message
		heartc <-chan time.Time

		tickc = time.Tick(ConnReadTimeout / 3)
	)

	logger.Infof("started streamWriter to peer %s", sw.peerID)
	for {
		// select doesn't select nil channel
		// if multiple cases are available, it selects randomly
		select {
		case conn := <-sw.connc:
			messageBinaryEncoder = raftpb.NewMessageBinaryEncoder(conn.Writer)
			httpFlusher = conn.Flusher

			sw.mu.Lock()
			closed := sw.unsafeCloseWriter()
			sw.status.activate()
			sw.closer = conn.Closer
			sw.working = true
			sw.mu.Unlock()

			if closed {
				logger.Warningf("closed an existing streamWriter to peer %s", sw.peerID)
			}
			logger.Infof("established streamWriter to peer %s", sw.peerID)
			msgc, heartc = sw.msgc, tickc

		case msg := <-msgc:
			err := messageBinaryEncoder.Encode(&msg)
			if err == nil {
				// no message is left in the channel
				// or buffered(batched) streams are beyond the half of buffer size
				if len(msgc) == 0 || batchedN > streamBufferN/2 {
					httpFlusher.Flush()
					batchedN = 0
				} else { // do not flush yet
					batchedN++
				}
				continue
			}

			// error, so deactivate
			sw.status.deactivate(failureType{source: "streamWriter message", action: "encode message", err: err})

			logger.Warningf("failed to encode message; closing streamWriter to peer %s (%v)", sw.peerID, err)
			sw.closeWriter()
			msgc, heartc = nil, nil // so that, 'select' doesn't select these cases
			sw.r.ReportUnreachable(msg.To)

		case <-heartc:
			err := messageBinaryEncoder.Encode(&emptyLeaderHeartbeat)
			if err == nil {
				httpFlusher.Flush()
				batchedN = 0
				continue
			}

			// error, so deactivate
			sw.status.deactivate(failureType{source: "streamWriter message", action: "encode heartbeat", err: err})

			logger.Warningf("failed to encode heartbeat; closing streamWriter to peer %s (%v)", sw.peerID, err)
			sw.closeWriter()
			msgc, heartc = nil, nil // so that, 'select' doesn't select these cases

		case <-sw.stopc:
			sw.closeWriter()
			close(sw.donec)
			return
		}
	}
}

// (etcd rafthttp.streamWriter.attach)
func (sw *streamWriter) attachOutgoingConn(conn *outgoingConn) bool {
	select {
	case sw.connc <- conn:
		return true
	case <-sw.donec:
		return false
	}
}

// (etcd rafthttp.streamWriter.writec)
func (sw *streamWriter) messageChanToSend() (chan<- raftpb.Message, bool) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	return sw.msgc, sw.working
}
