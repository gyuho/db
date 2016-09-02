package ioutil

import "io"

// PageWriter implements io.Writer interface
// where writes are in page chunks when flushing.
// It only writes by chunks of 'pageBytesN'.
// That is, (*PageWriter).Write(p []byte) only writes
// where len(p) % pageBytesN == 0.
//
// (etcd pkg.ioutil.PageWriter)
type PageWriter struct {
	w io.Writer

	// pageBytesN is the number of bytes per page.
	// Then PageWriter splits bytes by the chunks of pageBytesN.
	pageBytesN int
	// pageOffset tracks the page offset of the base of the buffer.
	pageOffset int

	// buffered holds write buffer.
	buffered []byte
	// bufferedN counts the number of bytes pending
	// for writing in the buffer.
	bufferedN int
	// bufferedWatermark is the number of bytes that the buffer
	// can hold before it needs to be flushed. It is less than len(buf)
	// so there is spacefor slack writes to bring the writer to page
	// alignment.
	bufferedWatermark int
}

var defaultBufferBytesN = 128 * 1024

// NewPageWriter creates a new PageWriter with specified number
// of bytes per page.
func NewPageWriter(w io.Writer, pageBytesN int) *PageWriter {
	return &PageWriter{
		w: w,

		pageBytesN: pageBytesN,
		pageOffset: 0,

		buffered:          make([]byte, defaultBufferBytesN+pageBytesN), // ???
		bufferedN:         0,
		bufferedWatermark: defaultBufferBytesN,
	}
}

func (pw *PageWriter) Flush() error {
	if pw.bufferedN == 0 {
		return nil
	}
	_, err := pw.w.Write(pw.buffered[:pw.bufferedN])
	pw.pageOffset = (pw.pageOffset + pw.bufferedN) % pw.pageBytesN
	pw.bufferedN = 0
	return err
}

// Write only writes where len(p) % pageBytesN == 0.
func (pw *PageWriter) Write(p []byte) (n int, err error) {
	if pw.bufferedN+len(p) <= pw.bufferedWatermark {
		// still have enough space in buffer
		// just put them in buffer
		copy(pw.buffered[pw.bufferedN:], p)
		pw.bufferedN += len(p)
		return len(p), nil
	}

	slackN := pw.pageBytesN - ((pw.pageOffset + pw.bufferedN) % pw.pageBytesN)
	// ???
	// len(p) % pw.pageBytesN == 0

	// buffers are not page-aligned
	if slackN != pw.pageBytesN {
		// complete slack page in the buffer
		partial := slackN > len(p)
		if partial { // ???
			slackN = len(p)
		}
		// slackN >= len(p)

		copy(pw.buffered[pw.bufferedN:], p[:slackN])
		pw.bufferedN += slackN
		n = slackN
		p = p[slackN:]

		if partial {
			// avoid forcing an unaligned flush
			return n, nil
		}
	}

	// buffers are page-aligned
	if err = pw.Flush(); err != nil {
		return
	}

	// directly write all complete pages without copying
	if len(p) > pw.pageBytesN {
		pagesN := len(p) / pw.pageBytesN
		boundary := pagesN * pw.pageBytesN
		c, werr := pw.w.Write(p[:boundary])
		n += c
		if werr != nil {
			return n, werr
		}
		p = p[boundary:]
	}

	// write remaining tail to buffer
	c, werr := pw.Write(p)
	n += c
	return n, werr
}
