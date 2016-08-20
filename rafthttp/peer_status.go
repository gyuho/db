package rafthttp

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/gyuho/db/pkg/types"
)

// peerStatus is the status of remote peer in local member's viewpoint.
//
// (etcd rafthttp.peerStatus)
type peerStatus struct {
	id types.ID

	mu sync.Mutex
	// since is zero when the peer is not active.
	since time.Time
}

func newPeerStatus(id types.ID) *peerStatus {
	return &peerStatus{id: id}
}

func (s *peerStatus) activate() {
	s.mu.Lock()
	if s.since.IsZero() {
		logger.Infof("member %s became active", s.id)
		s.since = time.Now()
	}
	s.mu.Unlock()
}

func (s *peerStatus) deactivate(ft failureType) {
	s.mu.Lock()
	if !s.since.IsZero() {
		logger.Infof("member %s became inactive (%s)", s.id, ft.String())
		s.since = time.Time{}
	}
	s.mu.Unlock()
}

type failureType struct {
	Source string `json:"source"`
	Action string `json:"action"`
	Err    string `json:"error"`
}

func (ft failureType) String() string {
	buf := bytes.NewBuffer(nil)
	json.NewEncoder(buf).Encode(ft)
	return strings.TrimSpace(buf.String())
}

func (s *peerStatus) isActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.since.IsZero()
}

func (s *peerStatus) activeSince() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.since
}
