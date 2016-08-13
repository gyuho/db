package probing

import (
	"sync"
	"time"
)

// Status defines status check operation.
type Status interface {
	Total() int64
	Loss() int64

	Health() bool
	Err() error

	// SRTT returns estimated smoothed round trip time.
	SRTT() time.Duration

	// ClockDiff returns estimated clock difference.
	ClockDiff() time.Duration

	StopNotify() <-chan struct{}
}

type status struct {
	mu        sync.Mutex
	total     int64
	loss      int64
	health    bool
	err       error
	srtt      time.Duration
	clockDiff time.Duration

	stopc chan struct{}
}

// α(alpha) is weight factor for SRTT
//
// https://tools.ietf.org/html/rfc2988
// SRTT <- (1 - alpha) * SRTT + alpha * RTT
const α = 0.125

func (s *status) record(rtt time.Duration, when time.Time) {
	s.mu.Lock()

	s.total++

	s.health = true
	s.err = nil

	s.srtt = time.Duration((1-α)*float64(s.srtt) + α*float64(rtt))
	s.clockDiff = time.Now().Sub(when) - s.srtt/2

	s.mu.Unlock()
}
