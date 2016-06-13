package wal

import (
	"bytes"
	"hash/crc32"
	"io"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/gyuho/distdb/walpb"
)

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

		expectedRecord *walpb.Record
		expectedError  error
	}{
		{
			infoRecord,
			&walpb.Record{
				Type: 1,
				CRC:  crc32.Checksum(infoData, crcTable),
				Data: infoData,
			},
			nil,
		},
		{
			[]byte(""),
			&walpb.Record{},
			io.EOF,
		},
		{
			infoRecord[:8],
			&walpb.Record{},
			io.ErrUnexpectedEOF,
		},
		{
			infoRecord[:len(infoRecord)-len(infoData)-8],
			&walpb.Record{},
			io.ErrUnexpectedEOF,
		},
		{
			infoRecord[:len(infoRecord)-len(infoData)],
			&walpb.Record{},
			io.ErrUnexpectedEOF,
		},
		{
			infoRecord[:len(infoRecord)-8],
			&walpb.Record{},
			io.ErrUnexpectedEOF,
		},
		{
			badInfoRecord,
			&walpb.Record{},
			walpb.ErrCRCMismatch,
		},
	}

	for i, tt := range tests {
		rc := ioutil.NopCloser(bytes.NewBuffer(tt.data))
		dec := newDecoder(rc)

		rec := &walpb.Record{}
		err := dec.decode(rec)

		if !reflect.DeepEqual(rec, tt.expectedRecord) {
			t.Fatalf("#%d: record expected %+v, got %+v", i, tt.expectedRecord, rec)
		}
		if !reflect.DeepEqual(err, tt.expectedError) {
			t.Fatalf("#%d: error expected %+v, got %+v", i, tt.expectedError, err)
		}
	}
}
