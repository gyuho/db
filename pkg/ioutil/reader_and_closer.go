package ioutil

import "io"

// ReaderAndCloser implements io.ReadCloser interface by combining
// reader and closer together.
//
// (etcd ioutil.ReaderAndCloser)
type ReaderAndCloser struct {
	io.Reader
	io.Closer
}
