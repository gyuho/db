package raftwal

import (
	"bytes"
	"hash/crc32"
	"io"
	"io/ioutil"
	"reflect"
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

var (
	infoData   = []byte("\b\xef\xfd\x02")
	infoRecord = append([]byte("\x0e\x00\x00\x00\x00\x00\x00\x00\b\x01\x10\x99\xb5\xe4\xd0\x03\x1a\x04"), infoData...)
)

func TestRecordDecode(t *testing.T) {
	badInfoRecord := make([]byte, len(infoRecord))
	copy(badInfoRecord, infoRecord)
	badInfoRecord[len(badInfoRecord)-1] = 'a'

	tests := []struct {
		data []byte

		expectedRecord *raftwalpb.Record
		expectedError  error
	}{
		{
			infoRecord,
			&raftwalpb.Record{
				Type: 1,
				CRC:  crc32.Checksum(infoData, crcTable),
				Data: infoData,
			},
			nil,
		},
		{
			[]byte(""),
			&raftwalpb.Record{},
			io.EOF,
		},
		{
			infoRecord[:8],
			&raftwalpb.Record{},
			io.ErrUnexpectedEOF,
		},
		{
			infoRecord[:len(infoRecord)-len(infoData)-8],
			&raftwalpb.Record{},
			io.ErrUnexpectedEOF,
		},
		{
			infoRecord[:len(infoRecord)-len(infoData)],
			&raftwalpb.Record{},
			io.ErrUnexpectedEOF,
		},
		{
			infoRecord[:len(infoRecord)-8],
			&raftwalpb.Record{},
			io.ErrUnexpectedEOF,
		},
		{
			badInfoRecord,
			&raftwalpb.Record{},
			raftwalpb.ErrCRCMismatch,
		},
	}

	for i, tt := range tests {
		rc := ioutil.NopCloser(bytes.NewBuffer(tt.data))
		dec := newDecoder(rc)

		rec := &raftwalpb.Record{}
		err := dec.decode(rec)

		if !reflect.DeepEqual(rec, tt.expectedRecord) {
			t.Fatalf("#%d: record expected %+v, got %+v", i, tt.expectedRecord, rec)
		}
		if !reflect.DeepEqual(err, tt.expectedError) {
			t.Fatalf("#%d: error expected %+v, got %+v", i, tt.expectedError, err)
		}
	}
}
