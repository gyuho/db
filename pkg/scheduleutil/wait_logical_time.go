package scheduleutil

import "sync"

// WaitLogicalTime defines wait operations with deadline.
//
// (etcd pkg.wait.WaitTime)
type WaitLogicalTime interface {
	// Wait returns a chan that waits on the given logical deadline.
	// The chan will be triggered when Trigger is called with a
	// deadline that is later than the one it is waiting for.
	// The given deadline MUST be unique. The deadline should be
	// retrieved by calling time.Now() in most cases.
	Wait(deadline uint64) <-chan struct{}

	// Trigger triggers all the waiting chans with an earlier deadline.
	Trigger(deadline uint64)
}

var closec = make(chan struct{})

func init() { close(closec) }

// (etcd pkg.wait.timeList)
type waitListLogicalTime struct {
	mu                  sync.Mutex
	lastTriggerDeadline uint64
	list                map[uint64]chan struct{}
}

// NewWaitLogicalTime returns WaitLogicalTime with list.
//
// (etcd pkg.wait.NewTimeList)
func NewWaitLogicalTime() WaitLogicalTime {
	return &waitListLogicalTime{list: make(map[uint64]chan struct{})}
}

func (w *waitListLogicalTime) Wait(deadline uint64) <-chan struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.lastTriggerDeadline >= deadline {
		return closec
	}

	ch := w.list[deadline]
	if ch == nil {
		ch = make(chan struct{})
		w.list[deadline] = ch
	}
	return ch
}

func (w *waitListLogicalTime) Trigger(deadline uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.lastTriggerDeadline = deadline
	for d, ch := range w.list {
		if d <= deadline {
			delete(w.list, d)
			close(ch)
		}
	}
}
