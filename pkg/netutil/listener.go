package netutil

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
)

/*
https://golang.org/pkg/net/#Listener

type net.Listener interface {
        // Accept waits for and returns the next connection to the listener.
        Accept() (Conn, error)

        // Close closes the listener.
        // Any blocked Accept operations will be unblocked and return errors.
        Close() error

        // Addr returns the listener's network address.
        Addr() Addr
}
*/

type listenerUnix struct{ net.Listener }

func (lu *listenerUnix) Close() error {
	if err := os.RemoveAll(lu.Addr().String()); err != nil {
		return err
	}
	return lu.Listener.Close()
}

// NewListenerUnix returns new net.Listener with unix socket
// (unix sockets via unix://address).
//
// (etcd pkg.transport.NewUnixListener)
func NewListenerUnix(addr string) (net.Listener, error) {
	if err := os.RemoveAll(addr); err != nil {
		return nil, err
	}
	l, err := net.Listen("unix", addr)
	if err != nil {
		return nil, err
	}
	return &listenerUnix{l}, nil
}

// NewListener returns new net.Listener based on the scheme and tls.Config.
//
// (etcd pkg.transport.NewListener)
func NewListener(addr, scheme string, tlsConfig *tls.Config) (l net.Listener, err error) {
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

	if scheme != "https" && scheme != "unixs" { // no need TLS
		return
	}
	if tlsConfig == nil { // need TLS, but empty config
		return nil, fmt.Errorf("cannot listen on TLS for %s: KeyFile and CertFile are not presented", scheme+"://"+addr)
	}

	return tls.NewListener(l, tlsConfig), nil
}
