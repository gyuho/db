package rafthttp

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"

	"github.com/gyuho/db/pkg/types"
)

// remoteMemberStatus is the status of remote member in local member's viewpoint.
//
// (etcd rafthttp.peerStatus)
type remoteMemberStatus struct {
	id types.ID

	mu sync.Mutex

	// since is zero when the remote member is not active
	since time.Time
}

func newRemoteMemberStatus(id types.ID) *remoteMemberStatus {
	return &remoteMemberStatus{id: id}
}

func (s *remoteMemberStatus) activate() {
	s.mu.Lock()
	if s.since.IsZero() {
		logger.Infof("remote member %s became active", s.id)
		s.since = time.Now()
	}
	s.mu.Unlock()
}

func (s *remoteMemberStatus) deactivate(ft failureType) {
	s.mu.Lock()
	if !s.since.IsZero() {
		logger.Infof("remote member %s became inactive (%s)", s.id, ft.String())
		s.since = time.Time{}
	}
	s.mu.Unlock()
}

type failureType struct {
	source string `json:"source"`
	action string `json:"action"`
	err    error  `json:"error"`
}

func (ft failureType) String() string {
	w := bytes.NewBuffer(nil)
	json.NewEncoder(w).Encode(ft)
	return w.String()
}

func (s *remoteMemberStatus) isActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.since.IsZero()
}

func (s *remoteMemberStatus) activeSince() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.since
}
