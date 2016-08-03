package netutil

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

// (etcd pkg.transport.timeoutConn)
type connWithTimeout struct {
	net.Conn
	writeTimeout time.Duration
	readTimeout  time.Duration
}

func (c *connWithTimeout) Write(b []byte) (n int, err error) {
	if c.writeTimeout > 0 {
		if err := c.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
			return 0, err
		}
	}
	return c.Conn.Write(b)
}

func (c *connWithTimeout) Read(b []byte) (n int, err error) {
	if c.readTimeout > 0 {
		if err := c.SetReadDeadline(time.Now().Add(c.readTimeout)); err != nil {
			return 0, err
		}
	}
	return c.Conn.Read(b)
}

// (etcd pkg.transport.rwTimeoutListener)
type listenerWithTimeout struct {
	net.Listener
	writeTimeout time.Duration
	readTimeout  time.Duration
}

func (l *listenerWithTimeout) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &connWithTimeout{
		Conn:         c,
		writeTimeout: l.writeTimeout,
		readTimeout:  l.readTimeout,
	}, nil
}

// NewListenerWithTimeout returns Listener that listens on the given address.
// If read/write on the accepted connection blocks longer than its time limit,
// it will return timeout error.
//
// (etcd pkg.transport.NewTimeoutListener)
func NewListenerWithTimeout(addr, scheme string, tlsConfig *tls.Config, writeTimeout, readTimeout time.Duration) (l net.Listener, err error) {
	switch scheme {
	case "unix", "unixs":
		l, err = NewListenerUnix(addr)
		if err != nil {
			return
		}

	case "http", "https":
		l, err = net.Listen("tcp", addr)
		if err != nil {
			return
		}

	default:
		return nil, fmt.Errorf("%q is not supported", scheme)
	}

	l = &listenerWithTimeout{
		Listener:     l,
		writeTimeout: writeTimeout,
		readTimeout:  readTimeout,
	}

	if scheme != "https" && scheme != "unixs" { // no need TLS
		return
	}
	if tlsConfig == nil { // need TLS, but empty config
		return nil, fmt.Errorf("cannot listen on TLS for %s: KeyFile and CertFile are not presented", scheme+"://"+addr)
	}
	return tls.NewListener(l, tlsConfig), nil
}
