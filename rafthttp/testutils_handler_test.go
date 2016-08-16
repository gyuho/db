package rafthttp

import (
	"net/http"

	"github.com/gyuho/db/version"
)

//////////////////////////////////////////////////////////////

// (etcd rafthttp.fakeStreamHandler)
type fakeStreamHandler struct {
	sw *streamWriter
}

func (h *fakeStreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add(HeaderServerVersion, version.ServerVersion)
	w.(http.Flusher).Flush()
	c := newCloseNotifier()
	h.sw.attachOutgoingConn(&outgoingConn{
		Writer:  w,
		Flusher: w.(http.Flusher),
		Closer:  c,
	})
	<-c.closeNotify()
}

//////////////////////////////////////////////////////////////

// (etcd rafthttp.syncHandler)
type syncHandler struct {
	h  http.Handler
	ch chan<- struct{}
}

func (sh *syncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sh.h.ServeHTTP(w, r)
	sh.ch <- struct{}{}
}

//////////////////////////////////////////////////////////////
