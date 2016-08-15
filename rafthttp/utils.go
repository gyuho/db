package rafthttp

// (etcd rafthttp.reportCriticalError)
func sendError(err error, errc chan<- error) {
	select {
	case errc <- err:
	default:
	}
}
