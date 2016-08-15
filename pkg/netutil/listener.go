package netutil

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
)

/*
https://golang.org/pkg/net/#Listener
A Listener is a generic network listener for stream-oriented protocols.
Multiple goroutines may invoke methods on a Listener simultaneously.

type net.Listener interface {
        // Accept waits for and returns the next connection to the listener.
        Accept() (Conn, error)

        // Close closes the listener.
        // Any blocked Accept operations will be unblocked and return errors.
        Close() error

        // Addr returns the listener's network address.
        Addr() Addr
}


https://golang.org/pkg/net/#Conn
Conn is a generic stream-oriented network connection.
Multiple goroutines may invoke methods on a Conn simultaneously.

type net.Conn interface {
        // Read reads data from the connection.
        // Read can be made to time out and return a Error with Timeout() == true
        // after a fixed time limit; see SetDeadline and SetReadDeadline.
        Read(b []byte) (n int, err error)

        // Write writes data to the connection.
        // Write can be made to time out and return a Error with Timeout() == true
        // after a fixed time limit; see SetDeadline and SetWriteDeadline.
        Write(b []byte) (n int, err error)

        // Close closes the connection.
        // Any blocked Read or Write operations will be unblocked and return errors.
        Close() error

        // LocalAddr returns the local network address.
        LocalAddr() Addr

        // RemoteAddr returns the remote network address.
        RemoteAddr() Addr

        // SetDeadline sets the read and write deadlines associated
        // with the connection. It is equivalent to calling both
        // SetReadDeadline and SetWriteDeadline.
        //
        // A deadline is an absolute time after which I/O operations
        // fail with a timeout (see type Error) instead of
        // blocking. The deadline applies to all future I/O, not just
        // the immediately following call to Read or Write.
        //
        // An idle timeout can be implemented by repeatedly extending
        // the deadline after successful Read or Write calls.
        //
        // A zero value for t means I/O operations will not time out.
        SetDeadline(t time.Time) error

        // SetReadDeadline sets the deadline for future Read calls.
        // A zero value for t means Read will not time out.
        SetReadDeadline(t time.Time) error

        // SetWriteDeadline sets the deadline for future Write calls.
        // Even if write times out, it may return n > 0, indicating that
        // some of the data was successfully written.
        // A zero value for t means Write will not time out.
        SetWriteDeadline(t time.Time) error
}


https://golang.org/pkg/net/http/#RoundTripper
RoundTripper is an interface representing the ability to execute a single HTTP transaction,
obtaining the Response for a given Request.
A RoundTripper must be safe for concurrent use by multiple goroutines.

type http.RoundTripper interface {
	RoundTrip(*Request) (*Response, error)
}

http.Transport implements this interface.
*/

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
