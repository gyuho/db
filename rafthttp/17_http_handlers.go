package rafthttp

/*
https://golang.org/pkg/net/http/#Handler

A Handler responds to an HTTP request.

ServeHTTP should write reply headers and data to the ResponseWriter and then return.
Returning signals that the request is finished; it is not valid to use the ResponseWriter
or read from the Request.Body after or concurrently with the completion of the ServeHTTP call.

type http.Handler interface {
        ServeHTTP(ResponseWriter, *Request)
}

HandlerFunc, ServeMux implement http.Handler



https://golang.org/pkg/net/http/#ResponseWriter

A ResponseWriter interface is used by an HTTP handler to construct an HTTP response.
A ResponseWriter may not be used after the Handler.ServeHTTP method has returned.

type http.ResponseWriter interface {
        // Header returns the header map that will be sent by
        // WriteHeader. Changing the header after a call to
        // WriteHeader (or Write) has no effect unless the modified
        // headers were declared as trailers by setting the
        // "Trailer" header before the call to WriteHeader (see example).
        // To suppress implicit response headers, set their value to nil.
        Header() Header

        // Write writes the data to the connection as part of an HTTP reply.
        // If WriteHeader has not yet been called, Write calls WriteHeader(http.StatusOK)
        // before writing the data.  If the Header does not contain a
        // Content-Type line, Write adds a Content-Type set to the result of passing
        // the initial 512 bytes of written data to DetectContentType.
        Write([]byte) (int, error)

        // WriteHeader sends an HTTP response header with status code.
        // If WriteHeader is not called explicitly, the first call to Write
        // will trigger an implicit WriteHeader(http.StatusOK).
        // Thus explicit calls to WriteHeader are mainly used to
        // send error codes.
        WriteHeader(int)
}



https://golang.org/pkg/net/http/#Flusher

The Flusher interface is implemented by ResponseWriters that
allow an HTTP handler to flush buffered data to the client.

Note that even for ResponseWriters that support Flush,
if the client is connected through an HTTP proxy,
the buffered data may not reach the client until the response completes.

type http.Flusher interface {
        // Flush sends any buffered data to the client.
        Flush()
}
*/

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

func (n *closeNotifier) closeNotify() <-chan struct{} { return n.donec }
