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
