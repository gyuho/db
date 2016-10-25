package ioutil

import (
	"fmt"
	"math/rand"
	"testing"
)

// checkPageWriter fails on unaligned page writes.
type checkPageWriter struct {
	t          *testing.T
	pageChunkN int

	writesN     int
	writeBytesN int
}

func (cw *checkPageWriter) Write(p []byte) (int, error) {
	if len(p)%cw.pageChunkN != 0 {
		cw.t.Fatalf("got unaligned page writes(%d); pageChunkN=%d", len(p), cw.pageChunkN)
	}
	cw.writesN++
	cw.writeBytesN += len(p)
	return len(p), nil
}

// (etcd pkg.ioutil.TestPageWriterRandom)
func Test_PageWriter_random(t *testing.T) {
	defaultWatermarkN = 8 * 1024
	pageChunkN := 128
	cw := &checkPageWriter{t: t, pageChunkN: pageChunkN}
	pw := NewPageWriter(cw, pageChunkN, 0)

	// randomly write empty bytes
	data := make([]byte, 4*defaultWatermarkN)

	n := 0
	for i := 0; i < 4*1024; i++ {
		c, err := pw.Write(data[:rand.Intn(len(data))])
		if err != nil {
			t.Fatal(err)
		}
		n += c
	}

	if cw.writeBytesN > n {
		t.Fatalf("writeBytesN expected %d, got %d", n, cw.writeBytesN)
	}
	pendingN := cw.writeBytesN - n
	if pendingN > pageChunkN {
		t.Fatalf("pending bytes got %d, expected <pageChunkN(%d)", pendingN, pageChunkN)
	}
	fmt.Println("total writes:", cw.writesN)
	fmt.Printf("total writes(bytes) flushed: %d out of %d\n", cw.writeBytesN, n)
}

// (etcd pkg.ioutil.TestPageWriterPartialSlack)
func Test_PageWriter_partial(t *testing.T) {
	defaultWatermarkN = 1024
	pageChunkN := 128

	cw := &checkPageWriter{t: t, pageChunkN: 64}
	pw := NewPageWriter(cw, pageChunkN, 0)

	// randomly write empty bytes
	data := make([]byte, defaultWatermarkN)

	// expecting only 1 write
	if _, err := pw.Write(data[:64]); err != nil {
		t.Fatal(err)
	}
	if err := pw.Flush(); err != nil {
		t.Fatal(err)
	}
	if cw.writesN != 1 {
		t.Fatalf("writesN expected 1, got %d", cw.writesN)
	}

	// nearly fill up the buffer, after flush
	if _, err := pw.Write(data[:1022]); err != nil {
		t.Fatal(err)
	}
	// overflow the buffer, but not enough to create
	// another page alignment
	if _, err := pw.Write(data[:8]); err != nil {
		t.Fatal(err)
	}
	if cw.writesN != 1 {
		t.Fatalf("writesN expected 1, got %d", cw.writesN)
	}

	// finish aligned write
	if _, err := pw.Write(data[:pageChunkN]); err != nil {
		t.Fatal(err)
	}
	if cw.writesN != 2 {
		t.Fatalf("writesN expected 2, got %d", cw.writesN)
	}
}
