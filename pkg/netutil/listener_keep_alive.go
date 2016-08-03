package netutil

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

// connKeepAlive defines keep alive connection interface.
// connLimit implements connKeepAlive interface.
//
// (etcd pkg.transport.keepAliveConn)
type connKeepAlive interface {
	SetKeepAlive(bool) error
	SetKeepAlivePeriod(d time.Duration) error
}

// (etcd pkg.transport.keepaliveListener)
type listenerKeepAlive struct {
	net.Listener
}

func (l *listenerKeepAlive) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	kac := c.(connKeepAlive)

	// detection time: tcp_keepalive_time + tcp_keepalive_probes + tcp_keepalive_intvl
	// default on linux:  30 + 8 * 30
	// default on osx:    30 + 8 * 75
	kac.SetKeepAlive(true)
	kac.SetKeepAlivePeriod(30 * time.Second)

	return c, nil
}

// NewListenerKeepAlive returns a listener that listens on the given address.
// Be careful when wrap around KeepAliveListener with another Listener if TLSInfo is not nil.
// Some pkgs (like go/http) might expect Listener to return TLSConn type to start TLS handshake.
// http://tldp.org/HOWTO/TCP-Keepalive-HOWTO/overview.html
//
// (etcd pkg.transport.NewKeepAliveListener)
func NewListenerKeepAlive(l net.Listener, scheme string, tlsConfig *tls.Config) (net.Listener, error) {
	if scheme == "https" {
		if tlsConfig == nil {
			return nil, fmt.Errorf("cannot listen on TLS for given listener: KeyFile and CertFile are not presented")
		}
		return NewListenerKeepAliveTLS(l, tlsConfig), nil
	}
	return &listenerKeepAlive{Listener: l}, nil
}

// (etcd pkg.transport.tlsKeepaliveListener)
type listenerKeepAliveTLS struct {
	net.Listener
	tlsConfig *tls.Config
}

func (l *listenerKeepAliveTLS) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	kac := c.(connKeepAlive)

	// detection time: tcp_keepalive_time + tcp_keepalive_probes + tcp_keepalive_intvl
	// default on linux:  30 + 8 * 30
	// default on osx:    30 + 8 * 75
	kac.SetKeepAlive(true)
	kac.SetKeepAlivePeriod(30 * time.Second)

	c = tls.Server(c, l.tlsConfig)
	return c, nil
}

// NewListenerKeepAliveTLS creates a Listener which accepts connections from an inner
// Listener and wraps each connection with Server.
// The configuration config must be non-nil and must have
// at least one certificate.
//
// (etcd pkg.transport.newTLSKeepaliveListener)
func NewListenerKeepAliveTLS(l net.Listener, tlsConfig *tls.Config) net.Listener {
	ln := &listenerKeepAliveTLS{}
	ln.Listener = l
	ln.tlsConfig = tlsConfig
	return ln
}
