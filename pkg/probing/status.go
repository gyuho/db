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

func (s *status) record(rtt time.Duration, requested time.Time) {
	s.mu.Lock()

	s.total++

	s.health = true
	s.err = nil

	s.srtt = time.Duration((1-α)*float64(s.srtt) + α*float64(rtt))
	s.clockDiff = time.Now().Sub(requested) - s.srtt/2

	s.mu.Unlock()
}

func (s *status) recordFailure(err error) {
	s.mu.Lock()

	s.total++
	s.loss++

	s.health = false
	s.err = err

	s.mu.Unlock()
}

func (s *status) reset() {
	s.mu.Lock()

	s.total = 0
	s.loss = 0

	s.health = false
	s.err = nil

	s.srtt = 0
	s.clockDiff = 0

	s.mu.Unlock()
}

func (s *status) Total() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.total
}

func (s *status) Loss() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loss
}

func (s *status) Health() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.health
}

func (s *status) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *status) SRTT() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.srtt
}

func (s *status) ClockDiff() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clockDiff
}

func (s *status) StopNotify() <-chan struct{} {
	return s.stopc
}
