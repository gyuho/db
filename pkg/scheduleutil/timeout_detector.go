package scheduleutil

import (
	"sync"
	"time"
)

// TimeoutDetector detects routine starvations by observing the
// time to finish an action with expected interval.
//
// (etcd pkg.contention.TimeoutDetector)
type TimeoutDetector struct {
	mu sync.Mutex

	timeout time.Duration
	records map[uint64]time.Time
}

// NewTimeoutDetector returns a new TimeoutDetector.
//
// (etcd pkg.contention.NewTimeoutDetector)
func NewTimeoutDetector(timeout time.Duration) *TimeoutDetector {
	return &TimeoutDetector{
		timeout: timeout,
		records: make(map[uint64]time.Time),
	}
}

// Reset resets TimeoutDetector.
//
// (etcd pkg.contention.TimeoutDetector.Reset)
func (td *TimeoutDetector) Reset() {
	td.mu.Lock()
	td.records = make(map[uint64]time.Time)
	td.mu.Unlock()
}

// Observe observes an event of given id, and returns false and exceeded duration
// if the event took less than the time-out.
//
// (etcd pkg.contention.TimeoutDetector.Observe)
func (td *TimeoutDetector) Observe(id uint64) (bool, time.Duration) {
	beyondTimeout, exceeded := true, time.Duration(0)
	now := time.Now()

	td.mu.Lock()
	if tm, ok := td.records[id]; ok {
		exceeded = now.Sub(tm) - td.timeout
		if exceeded > 0 {
			beyondTimeout = false
		}
	}
	td.records[id] = now
	td.mu.Unlock()

	return beyondTimeout, exceeded
}
