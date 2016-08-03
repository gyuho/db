package scheduleutil

import (
	"sync"
	"time"
)

// WaitWithDeadline defines wait operations with deadline.
//
// (etcd pkg.wait.WaitTime)
type WaitWithDeadline interface {
	// Wait returns a chan that waits on the given deadline.
	// The chan will be triggered when Trigger is called with a
	// deadline that is later than the one it is waiting for.
	// The given deadline MUST be unique. The deadline should be
	// retrieved by calling time.Now() in most cases.
	Wait(deadline time.Time) <-chan struct{}

	// Trigger triggers all the waiting chans with an earlier deadline.
	Trigger(deadline time.Time)
}

type waitListWithDeadline struct {
	mu   sync.Mutex
	list map[int64]chan struct{}
}

// NewWaitWithDeadline returns WaitWithDeadline with list.
//
// (etcd pkg.wait.NewTimeList)
func NewWaitWithDeadline() WaitWithDeadline {
	return &waitListWithDeadline{list: make(map[int64]chan struct{})}
}

func (w *waitListWithDeadline) Wait(deadline time.Time) <-chan struct{} {
	ch := make(chan struct{}, 1)

	w.mu.Lock()
	w.list[deadline.UnixNano()] = ch // The given deadline SHOULD be unique.
	w.mu.Unlock()

	return ch
}

func (w *waitListWithDeadline) Trigger(deadline time.Time) {
	w.mu.Lock()
	for t, ch := range w.list {
		if t < deadline.UnixNano() {
			delete(w.list, t)
			close(ch)
		}
	}
	w.mu.Unlock()
}
