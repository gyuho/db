package netutil

import (
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

// NewListenerUnix returns new net.Listener with unix socket.
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
