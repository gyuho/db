package netutil

/*
https://golang.org/pkg/net/#Conn
Conn is a generic stream-oriented network connection.
Multiple goroutines may invoke methods on a Conn simultaneously.

func net.Dial(network, address string) (net.Conn, error)
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



https://golang.org/pkg/net/http/#RoundTripper
RoundTripper is an interface representing the ability to execute a single HTTP transaction,
obtaining the Response for a given Request.
A RoundTripper must be safe for concurrent use by multiple goroutines.

type http.RoundTripper interface {
	RoundTrip(*Request) (*Response, error)
}

http.Transport implements this interface.



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
