package rafthttp

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gyuho/db/pkg/netutil"
	"github.com/gyuho/db/pkg/tlsutil"
)

// NewStreamListener returns a listener for raft message transfer between peers.
// It uses timeout listener to identify broken streams promptly.
//
// (etcd rafthttp.NewListener)
func NewStreamListener(u url.URL, tlsConfig *tls.Config) (net.Listener, error) {
	return netutil.NewListenerWithTimeout(u.Host, u.Scheme, tlsConfig, ConnWriteTimeout, ConnReadTimeout)
}

// NewStreamRoundTripper returns a roundTripper used to send requests to its peers.
//
// Read/Write timeout is set for stream roundTripper to promptly find out broken status,
// which minimizes the number of messages sent on broken connection.
//
// (etcd rafthttp.newStreamRoundTripper)
func NewStreamRoundTripper(ti tlsutil.TLSInfo, dialTimeout time.Duration) (http.RoundTripper, error) {
	return netutil.NewTransportWithTimeout(ti, dialTimeout, ConnWriteTimeout, ConnReadTimeout)
}

// NewRoundTripper returns a roundTripper used to send requests to its peers.
//
// (etcd rafthttp.NewRoundTripper)
func NewRoundTripper(ti tlsutil.TLSInfo, dialTimeout time.Duration) (http.RoundTripper, error) {
	writeTimeout, readTimeout := time.Duration(0), time.Duration(0) // to allow more idle connections
	return netutil.NewTransportWithTimeout(ti, dialTimeout, writeTimeout, readTimeout)
}
