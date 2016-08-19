package rafthttp

import (
	"net/http"
	"time"

	"github.com/gyuho/db/pkg/types"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftsnap"
)

// (etcd rafthttp.nopTransporter)
type nopTransporter struct{}

func NewNopTransporter() Transporter {
	return &nopTransporter{}
}

func (s *nopTransporter) Start() error                           { return nil }
func (s *nopTransporter) HTTPHandler() http.Handler              { return nil }
func (s *nopTransporter) Send(m []raftpb.Message)                {}
func (s *nopTransporter) SendSnapshot(msg raftsnap.Message)      {}
func (s *nopTransporter) AddPeerRemote(id types.ID, us []string) {}
func (s *nopTransporter) AddPeer(id types.ID, us []string)       {}
func (s *nopTransporter) RemovePeer(id types.ID)                 {}
func (s *nopTransporter) RemoveAllPeers()                        {}
func (s *nopTransporter) UpdatePeer(id types.ID, us []string)    {}
func (s *nopTransporter) ActiveSince(id types.ID) time.Time      { return time.Time{} }
func (s *nopTransporter) Stop()                                  {}
func (s *nopTransporter) Pause()                                 {}
func (s *nopTransporter) Resume()                                {}
