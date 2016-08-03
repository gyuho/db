package netutil

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gyuho/db/pkg/tlsutil"
)

/*
type http.RoundTripper interface {
	RoundTrip(*Request) (*Response, error)
}

http.Transport implements this interface.
*/

// (etcd pkg.transport.unixListener)
type transportUnix struct{ *http.Transport }

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
