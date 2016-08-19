package rafthttp

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/version"
)

// (etcd rafthttp.writerToResponse)
type writerToResponse interface {
	WriteTo(rw http.ResponseWriter)
}

// (etcd rafthttp.setPeerURLsHeader)
func setHeaderPeerURLs(req *http.Request, peerURLs types.URLs) {
	if peerURLs == nil {
		return
	}
	ps := make([]string, peerURLs.Len())
	for i := range peerURLs {
		ps[i] = peerURLs[i].String()
	}
	req.Header.Set(HeaderPeerURLs, strings.Join(ps, ","))
}

// (etcd rafthttp.createPostRequest)
func createPostRequest(target url.URL, path string, rd io.Reader, contentType string, from, clusterID types.ID, peerURLs types.URLs) *http.Request {
	uu := target
	uu.Path = path

	req, err := http.NewRequest("POST", uu.String(), rd)
	if err != nil {
		logger.Panic(err)
	}

	req.Header.Set(HeaderContentType, contentType)
	req.Header.Set(HeaderFromID, from.String())
	req.Header.Set(HeaderClusterID, clusterID.String())
	req.Header.Set(HeaderServerVersion, version.ServerVersion)

	setHeaderPeerURLs(req, peerURLs)

	return req
}

// (etcd rafthttp.checkPostResponse)
func checkPostResponse(resp *http.Response, body []byte, req *http.Request, peerID types.ID) error {
	switch resp.StatusCode {
	case http.StatusPreconditionFailed:
		switch strings.TrimSpace(string(body)) {
		case ErrClusterIDMismatch.Error():
			logger.Errorf("request was ignored (%v, remote[%s]=%s, local=%s)", ErrClusterIDMismatch, peerID, resp.Header.Get(HeaderClusterID), req.Header.Get(HeaderClusterID))
			return ErrClusterIDMismatch
		default:
			return fmt.Errorf("unhandled error %q", body)
		}

	case http.StatusForbidden:
		return ErrMemberRemoved

	case http.StatusNoContent:
		return nil

	default:
		return fmt.Errorf("unexpected http status %s while posting to %q", http.StatusText(resp.StatusCode), req.URL.String())
	}
}
