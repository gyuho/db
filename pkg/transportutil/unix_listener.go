package transportutil

import (
	"net"
	"os"
)

/*
// https://golang.org/pkg/net/#Listener
type Listener interface {
        // Accept waits for and returns the next connection to the listener.
        Accept() (Conn, error)

        // Close closes the listener.
        // Any blocked Accept operations will be unblocked and return errors.
        Close() error

        // Addr returns the listener's network address.
        Addr() Addr
}
*/

// NewUnixListener returns new net.Listener with unix socket.
func NewUnixListener(addr string) (net.Listener, error) {
	if err := os.RemoveAll(addr); err != nil {
		return nil, err
	}
	l, err := net.Listen("unix", addr)
	if err != nil {
		return nil, err
	}
	return &unixListener{l}, nil
}

type unixListener struct{ net.Listener }

func (ul *unixListener) Close() error {
	if err := os.RemoveAll(ul.Addr().String()); err != nil {
		return err
	}
	return ul.Listener.Close()
}
