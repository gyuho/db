package rafthttp

import (
	"time"

	"github.com/gyuho/db/pkg/probing"
)

var (
	// proberInterval must be shorter than read timeout.
	// Or the connection will time-out.
	proberInterval           = ConnReadTimeout - time.Second
	statusMonitoringInterval = 30 * time.Second
	statusErrorInterval      = 5 * time.Second
)

func addPeerToProber(p probing.Prober, id string, us []string) {
	hus := make([]string, len(us))
	for i := range us {
		hus[i] = us[i] + PrefixRaftProbing
	}

	p.AddHTTP(id, proberInterval, hus)

	s, err := p.Status(id)
	if err != nil {
		logger.Errorf("failed to add peer %s to prober", id)
		return
	}
	go monitorProbingStatus(s, id)
}

func monitorProbingStatus(s probing.Status, id string) {
	// set the first interval short to log error early.
	interval := statusErrorInterval

	for {
		select {
		case <-time.After(interval):
			if !s.Health() {
				logger.Warningf("health check for peer %s could not connect (%v)", id, s.Err())
				interval = statusErrorInterval
			} else {
				interval = statusMonitoringInterval
			}

			if s.ClockDiff() > time.Second {
				logger.Warningf("clock difference to peer %s is too high [%v > %v]", id, s.ClockDiff(), time.Second)
			}

		case <-s.StopNotify():
			return
		}
	}
}
