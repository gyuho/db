package rafthttp

// (etcd rafthttp.closeNotifier)
type closeNotifier struct{ donec chan struct{} }

// (etcd rafthttp.newCloseNotifier)
func newCloseNotifier() *closeNotifier {
	return &closeNotifier{donec: make(chan struct{})}
}

func (n *closeNotifier) Close() error {
	close(n.donec)
	return nil
}

func (n *closeNotifier) closeNotify() <-chan struct{} {
	return n.donec
}
