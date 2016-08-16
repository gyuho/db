package rafthttp

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/version"
)

// (etcd rafthttp.TestServeRaftPrefix)
func Test_pipelineHandler(t *testing.T) {
	emptyMsg := &raftpb.Message{}
	emptyMsgBts, err := emptyMsg.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		method    string
		body      io.Reader
		r         Raft
		clusterID string

		wcode int
	}{
		{ // bad method
			"GET",
			bytes.NewReader(emptyMsgBts),
			&fakeRaft{},
			"0",
			http.StatusMethodNotAllowed,
		},
		{ // bad method
			"PUT",
			bytes.NewReader(emptyMsgBts),
			&fakeRaft{},
			"0",
			http.StatusMethodNotAllowed,
		},
		{ // bad method
			"DELETE",
			bytes.NewReader(emptyMsgBts),
			&fakeRaft{},
			"0",
			http.StatusMethodNotAllowed,
		},
		{ // bad request body
			"POST",
			&errorReader{},
			&fakeRaft{},
			"0",
			http.StatusBadRequest,
		},
		{ // bad request protobuf
			"POST",
			strings.NewReader("malformed garbage"),
			&fakeRaft{},
			"0",
			http.StatusBadRequest,
		},
		{ // good request, wrong cluster ID
			"POST",
			bytes.NewReader(emptyMsgBts),
			&fakeRaft{},
			"1",
			http.StatusPreconditionFailed,
		},
		{ // good request, Processor failure
			"POST",
			bytes.NewReader(
				emptyMsgBts,
			),
			&fakeRaft{err: &fakeWriterToResponse{code: http.StatusForbidden}},
			"0",
			http.StatusForbidden,
		},
		{ // good request, Processor failure
			"POST",
			bytes.NewReader(
				emptyMsgBts,
			),
			&fakeRaft{err: &fakeWriterToResponse{code: http.StatusInternalServerError}},
			"0",
			http.StatusInternalServerError,
		},
		{ // good request, Processor failure
			"POST",
			bytes.NewReader(emptyMsgBts),
			&fakeRaft{err: errors.New("blah")},
			"0",
			http.StatusInternalServerError,
		},
		{ // good request
			"POST",
			bytes.NewReader(
				emptyMsgBts,
			),
			&fakeRaft{},
			"0",
			http.StatusNoContent,
		},
	}
	for i, tt := range tests {
		req, err := http.NewRequest(tt.method, "foo", tt.body)
		if err != nil {
			t.Fatalf("#%d: could not create request: %v", i, err)
		}
		req.Header.Set(HeaderClusterID, tt.clusterID)
		req.Header.Set(HeaderServerVersion, version.ServerVersion)

		rw := httptest.NewRecorder()
		hd := newPipelineHandler(NewNopTransporter(), tt.r, types.ID(0))

		// goroutine because the handler panics to disconnect on raft error
		donec := make(chan struct{})
		go func() {
			defer func() {
				recover()
				close(donec)
			}()

			hd.ServeHTTP(rw, req)
		}()
		<-donec

		if rw.Code != tt.wcode {
			t.Errorf("#%d: code expected %d, got %d, ", i, tt.wcode, rw.Code)
		}
	}
}
