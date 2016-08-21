package main

import (
	"net/http"

	"github.com/gyuho/db/pkg/netutil"
)

func (rnd *raftNode) startPeerHandler() {
	ln, err := netutil.NewListenerStoppable(rnd.advertisePeerURL.Scheme, rnd.advertisePeerURL.Host, nil, rnd.stopListenerc)
	if err != nil {
		logger.Panic(err)
	}

	srv := &http.Server{
		Handler: rnd.transport.HTTPHandler(),
	}
	err = srv.Serve(ln)
	select {
	case <-rnd.stopListenerc:
	default:
		logger.Fatalf("failed to serve (%v)", err)
	}
	<-rnd.donec
}
