package netutil

import (
	"io"
	"io/ioutil"
	"net/http"
)

// SetHTTPRequestCancel sets Cancel channel.
// (DEPRECATED. Use context instead.)
//
// (etcd pkg.httputil.RequestCanceler)
func SetHTTPRequestCancel(req *http.Request) func() {
	ch := make(chan struct{})
	req.Cancel = ch
	return func() { close(ch) }
}

// GracefulClose drains http.Response.Body until it hits EOF
// and closes it. This prevents TCP/TLS connections from closing,
// therefore available for reuse.
func GracefulClose(resp *http.Response) {
	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
}
