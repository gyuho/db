package netutil

import "net"

// IsClosedConnectionError returns true if the error is closed connection error.
//
// (etcd rafthttp.isClosedConnectionError)
func IsClosedConnectionError(err error) bool {
	operr, ok := err.(*net.OpError)
	return ok && operr.Err.Error() == "use of closed network connection"
}
