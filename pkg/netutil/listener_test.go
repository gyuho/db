package netutil

import "testing"

// (etcd pkg.transport.TestNewListenerUnixSocket)
func Test_NewListenerUnix(t *testing.T) {
	l, err := NewListener("testsocket-address", "unix", nil)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
}
