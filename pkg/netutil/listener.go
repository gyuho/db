package netutil

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
)

type listenerUnix struct {
	net.Listener
}

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
func NewListener(scheme, addr string, tlsConfig *tls.Config) (l net.Listener, err error) {
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

type listenerStoppable struct {
	net.Listener
	stopc <-chan struct{}
}

// NewListenerStoppable returns stoppable net.Listener.
func NewListenerStoppable(scheme, addr string, tlsConfig *tls.Config, stopc <-chan struct{}) (net.Listener, error) {
	ln, err := NewListener(scheme, addr, tlsConfig)
	if err != nil {
		return nil, err
	}
	ls := &listenerStoppable{
		Listener: ln,
		stopc:    stopc,
	}
	return ls, nil
}

// ErrListenerStopped is returned when the listener is stopped.
var ErrListenerStopped = errors.New("listener stopped")

func (ln *listenerStoppable) Accept() (net.Conn, error) {
	connc, errc := make(chan net.Conn, 1), make(chan error)
	go func() {
		conn, err := ln.Listener.Accept() // (X) ln.Accept()
		if err != nil {
			errc <- err
			return
		}
		connc <- conn
	}()

	select {
	case <-ln.stopc:
		return nil, ErrListenerStopped
	case err := <-errc:
		return nil, err
	case conn := <-connc:
		return conn, nil
	}
}
