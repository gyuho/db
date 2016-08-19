package netutil

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gyuho/db/pkg/tlsutil"
)

// (etcd pkg.transport.unixListener)
type transportUnix struct {
	*http.Transport
}

func (tu *transportUnix) RoundTrip(req *http.Request) (*http.Response, error) {
	url := *req.URL
	req.URL = &url
	req.URL.Scheme = strings.Replace(req.URL.Scheme, "unix", "http", 1)
	return tu.Transport.RoundTrip(req)
}

// NewTransport creates a new http.Transport.
//
// (etcd pkg.transport.NewTransport)
func NewTransport(ti tlsutil.TLSInfo, dialTimeout time.Duration) (*http.Transport, error) {
	tlsClientConfig, err := ti.ClientConfig()
	if err != nil {
		return nil, err
	}

	// https://golang.org/pkg/net/http/#RoundTripper
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         getNetDialer(dialTimeout).DialContext,
		TLSHandshakeTimeout: 10 * time.Second, // from http.DefaultTransport
		TLSClientConfig:     tlsClientConfig,
	}

	dialer := getNetDialer(dialTimeout)
	dialFunc := func(ctx context.Context, net, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", addr)
	}

	// https://golang.org/pkg/net/http/#RoundTripper
	utr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         dialFunc,
		TLSHandshakeTimeout: 10 * time.Second, // from http.DefaultTransport
		TLSClientConfig:     tlsClientConfig,
	}
	ut := &transportUnix{utr}

	tr.RegisterProtocol("unix", ut)
	tr.RegisterProtocol("unixs", ut)

	return tr, nil
}

func getNetDialer(d time.Duration) *net.Dialer {
	// https://golang.org/pkg/net/http/#RoundTripper
	return &net.Dialer{
		Timeout:   d,
		KeepAlive: 30 * time.Second, // from http.DefaultTransport
	}
}

// (etcd pkg.transport.rwTimeoutDialer)
type dialerWithTimeout struct {
	net.Dialer
	writeTimeout time.Duration
	readTimeout  time.Duration
}

func (d dialerWithTimeout) Dial(network, address string) (net.Conn, error) {
	conn, err := d.Dialer.Dial(network, address)
	return &connWithTimeout{
		Conn:         conn,
		writeTimeout: d.writeTimeout,
		readTimeout:  d.readTimeout,
	}, err
}

// NewTransportWithTimeout returns a transport created using the given TLS info.
// If read/write on the created connection blocks longer than its time limit,
// it will return timeout error.
// If read/write timeout is set, transport will not be able to reuse connection.
//
// (etcd pkg.transport.NewTimeoutTransport)
func NewTransportWithTimeout(info tlsutil.TLSInfo, dialTimeout, writeTimeout, readTimeout time.Duration) (*http.Transport, error) {
	tr, err := NewTransport(info, dialTimeout)
	if err != nil {
		return nil, err
	}

	if readTimeout != 0 || writeTimeout != 0 {
		// the timed out connection will timeout soon after it is idle.
		// it should not be put back to http transport as an idle connection for future usage.
		tr.MaxIdleConnsPerHost = -1
	} else {
		// allow more idle connections between peers to avoid unncessary port allocation.
		tr.MaxIdleConnsPerHost = 1024
	}

	tr.Dial = (&dialerWithTimeout{
		Dialer:       *getNetDialer(dialTimeout),
		writeTimeout: writeTimeout,
		readTimeout:  readTimeout,
	}).Dial
	return tr, nil
}
