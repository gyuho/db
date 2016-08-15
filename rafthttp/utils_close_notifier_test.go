package rafthttp

import "testing"

// (etcd rafthttp.TestCloseNotifier)
func Test_closeNotifier(t *testing.T) {
	c := newCloseNotifier()
	select {
	case <-c.closeNotify():
		t.Fatalf("received unexpected close notification")
	default:
	}
	c.Close()
	select {
	case <-c.closeNotify():
	default:
		t.Fatalf("failed to get close notification")
	}
}
