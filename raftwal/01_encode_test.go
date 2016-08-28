package raftwal

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/gyuho/db/raftwal/raftwalpb"
)

func Test_encode_decode_header(t *testing.T) {
	dataN1 := 102
	padBytesN1 := getPadBytesN(dataN1)
	headerN1 := encodeLen(dataN1, padBytesN1)

	br := &bytes.Buffer{}
	wordBuf := make([]byte, 8)
	if err := writeHeaderN(br, wordBuf, headerN1); err != nil {
		t.Fatal(err)
	}

	headerN2, err := readHeaderN(br)
	if err != nil {
		t.Fatal(err)
	}

	dataN2, padBytesN2 := decodeLen(headerN2)

	if int64(dataN1) != dataN2 {
		t.Fatalf("dataN expected %b, got %b", dataN1, dataN2)
	}

	if int64(padBytesN1) != padBytesN2 {
		t.Fatalf("padBytesN expected %b, got %b", padBytesN1, padBytesN2)
	}

	if headerN1 != uint64(headerN2) {
		t.Fatalf("headerN expected %b, got %b", headerN1, headerN2)
	}
}

func TestRecordEncode(t *testing.T) {
	var (
		data = []byte("Hello World!")
		buf  = new(bytes.Buffer)
		enc  = newEncoder(buf, 0)

		tp = int64(0xABCDE)
	)
	enc.encode(&raftwalpb.Record{
		Type: tp,
		Data: data,
	})
	enc.flush()

	dec := newDecoder(ioutil.NopCloser(buf))
	rec := &raftwalpb.Record{}
	if err := dec.decode(rec); err != nil {
		t.Fatal(err)
	}
	if rec.Type != tp {
		t.Fatalf("type expected %x, got %x", tp, rec.Type)
	}
	if !bytes.Equal(rec.Data, data) {
		t.Fatalf("data expected %q, got %q", data, rec.Data)
	}
}
