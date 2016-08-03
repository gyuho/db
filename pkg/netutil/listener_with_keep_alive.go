package netutil

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

// connWithKeepAlive defines keep alive connection interface.
// connLimit implements connWithKeepAlive interface.
//
// (etcd pkg.transport.keepAliveConn)
type connWithKeepAlive interface {
	SetKeepAlive(bool) error
	SetKeepAlivePeriod(d time.Duration) error
}

// (etcd pkg.transport.keepaliveListener)
type listenerWithKeepAlive struct {
	net.Listener
}

func (l *listenerWithKeepAlive) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	kac := c.(connWithKeepAlive)

	// detection time: tcp_keepalive_time + tcp_keepalive_probes + tcp_keepalive_intvl
	// default on linux:  30 + 8 * 30
	// default on osx:    30 + 8 * 75
	kac.SetKeepAlive(true)
	kac.SetKeepAlivePeriod(30 * time.Second)

	return c, nil
}

// NewListenerWithKeepAlive returns a listener that listens on the given address.
// Be careful when wrap around KeepAliveListener with another Listener if TLSInfo is not nil.
// Some pkgs (like go/http) might expect Listener to return TLSConn type to start TLS handshake.
// http://tldp.org/HOWTO/TCP-Keepalive-HOWTO/overview.html
//
// (etcd pkg.transport.NewKeepAliveListener)
func NewListenerWithKeepAlive(l net.Listener, scheme string, tlsConfig *tls.Config) (net.Listener, error) {
	if scheme == "https" {
		if tlsConfig == nil {
			return nil, fmt.Errorf("cannot listen on TLS for given listener: KeyFile and CertFile are not presented")
		}
		return NewListenerWithKeepAliveTLS(l, tlsConfig), nil
	}
	return &listenerWithKeepAlive{Listener: l}, nil
}

// (etcd pkg.transport.tlsKeepaliveListener)
type listenerWithKeepAliveTLS struct {
	net.Listener
	tlsConfig *tls.Config
}

func (l *listenerWithKeepAliveTLS) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	kac := c.(connWithKeepAlive)

	// detection time: tcp_keepalive_time + tcp_keepalive_probes + tcp_keepalive_intvl
	// default on linux:  30 + 8 * 30
	// default on osx:    30 + 8 * 75
	kac.SetKeepAlive(true)
	kac.SetKeepAlivePeriod(30 * time.Second)

	c = tls.Server(c, l.tlsConfig)
	return c, nil
}

// NewListenerWithKeepAliveTLS creates a Listener which accepts connections from an inner
// Listener and wraps each connection with Server.
// The configuration config must be non-nil and must have
// at least one certificate.
//
// (etcd pkg.transport.newTLSKeepaliveListener)
func NewListenerWithKeepAliveTLS(l net.Listener, tlsConfig *tls.Config) net.Listener {
	ln := &listenerWithKeepAliveTLS{}
	ln.Listener = l
	ln.tlsConfig = tlsConfig
	return ln
}
