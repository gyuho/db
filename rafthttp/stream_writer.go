package rafthttp

import (
	"io"
	"net/http"
)

// (etcd rafthttp.ongoingConn)
type ongoingConn struct {
	io.Writer
	http.Flusher
	io.Closer
}

// streamWriter writes messages to the attached outgoingConn.
//
// (etcd rafthttp.streamWriter)
type streamWriter struct {
}
