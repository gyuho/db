package wal

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeLengthFieldN(t *testing.T) {
	dataN1 := 102
	padBytesN1 := dataNToPadBytesN(dataN1)
	lengthFieldN1 := encodeLengthFieldN(dataN1, padBytesN1)

	br := &bytes.Buffer{}
	wordBuf := make([]byte, 8)
	if err := putLengthFieldN(br, wordBuf, lengthFieldN1); err != nil {
		t.Fatal(err)
	}

	lengthFieldN2, err := readLengthFieldN(br)
	if err != nil {
		t.Fatal(err)
	}

	dataN2, padBytesN2 := decodeLengthFieldN(lengthFieldN2)

	if int64(dataN1) != dataN2 {
		t.Fatalf("dataN expected %b, got %b", dataN1, dataN2)
	}

	if int64(padBytesN1) != padBytesN2 {
		t.Fatalf("padBytesN expected %b, got %b", padBytesN1, padBytesN2)
	}

	if lengthFieldN1 != uint64(lengthFieldN2) {
		t.Fatalf("lengthFieldN expected %b, got %b", lengthFieldN1, lengthFieldN2)
	}
}
