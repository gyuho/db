package ioutil

import "io"

// PageWriter implements io.Writer interface
// where writes are in page chunks when flushing.
// It only writes by chunks of 'pageChunkN'.
// That is, (*PageWriter).Write(p []byte) only writes
// where len(p) % pageChunkN == 0.
//
// (etcd pkg.ioutil.PageWriter)
type PageWriter struct {
	w io.Writer

	// pageChunkN is the number of bytes per page.
	// PageWriter splits bytes by the chunks of pageChunkN.
	//
	// (etcd pkg.ioutil.pageBytes)
	pageChunkN int

	// pageOffset tracks the page offset of the base of the buffer.
	//
	// (etcd pkg.ioutil.pageOffset)
	pageOffset int

	// buffered holds write buffer.
	//
	// (etcd pkg.ioutil.buf)
	buffered []byte

	// bufferedN counts the number of bytes pending
	// for writing in the buffer.
	//
	// (etcd pkg.ioutil.bufferedBytes)
	bufferedN int

	// bufferedWatermarkN is the number of bytes that the buffer
	// can hold before it needs to be flushed. It is less than len(buf)
	// so there is spacefor slack writes to bring the writer to page
	// alignment.
	//
	// (etcd pkg.ioutil.bufWatermarkBytes)
	bufferedWatermarkN int
}

var defaultWatermarkN = 128 * 1024

// NewPageWriter creates a new PageWriter with specified number
// of bytes per page.
func NewPageWriter(w io.Writer, pageChunkN int) *PageWriter {
	return &PageWriter{
		w: w,

		pageChunkN: pageChunkN,
		pageOffset: 0,

		// ???
		buffered:           make([]byte, defaultWatermarkN+pageChunkN),
		bufferedN:          0,
		bufferedWatermarkN: defaultWatermarkN,
	}
}

func (pw *PageWriter) Flush() error {
	if pw.bufferedN == 0 {
		return nil
	}
	_, err := pw.w.Write(pw.buffered[:pw.bufferedN])
	pw.pageOffset = (pw.pageOffset + pw.bufferedN) % pw.pageChunkN
	pw.bufferedN = 0
	return err
}

// Write only writes where len(p) % pageChunkN == 0.
func (pw *PageWriter) Write(p []byte) (n int, err error) {
	if pw.bufferedN+len(p) <= pw.bufferedWatermarkN {
		// still have enough space in buffer
		// just put them in buffer
		copy(pw.buffered[pw.bufferedN:], p)
		pw.bufferedN += len(p)
		return len(p), nil
	}

	offset := (pw.pageOffset + pw.bufferedN) % pw.pageChunkN
	if offset != 0 { // buffers are NOT page-aligned
		leftN := pw.pageChunkN - offset
		overflow := leftN > len(p)
		if overflow {
			leftN = len(p)
		}
		copy(pw.buffered[pw.bufferedN:], p[:leftN])
		pw.bufferedN += leftN
		n = leftN
		p = p[leftN:]
		if overflow { // avoid forcing an unaligned flush
			return n, nil
		}
	}
	// buffers are page-aligned; clear out buffer
	if err = pw.Flush(); err != nil {
		return
	}

	// directly write all complete pages without copying
	if len(p) > pw.pageChunkN {
		pagesN := len(p) / pw.pageChunkN
		boundary := pagesN * pw.pageChunkN
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
