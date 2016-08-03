package scheduleutil

import (
	"fmt"
	"sync"
)

// Wait defines wait-operation interface.
//
// (etcd pkg.wait.Wait)
type Wait interface {
	// Register returns receiver channel that can be used to wait
	// until the event of id gets triggered and receive the value from Trigger call.
	Register(id uint64) <-chan interface{}

	// Trigger triggers the event of id with x.
	// And the channel from Register method will receive x from Trigger.
	Trigger(id uint64, x interface{})

	// IsRegistered returns true if the id is already registered.
	IsRegistered(id uint64) bool
}

// waitList contains all waiting events.
//
// (etcd pkg.wait.List)
type waitList struct {
	mu   sync.Mutex
	list map[uint64]chan interface{}
}

// NewWait returns Wait with list.
//
// (etcd pkg.wait.New)
func NewWait() Wait {
	return &waitList{list: make(map[uint64]chan interface{})}
}

func (w *waitList) Register(id uint64) <-chan interface{} {
	w.mu.Lock()
	defer w.mu.Unlock()

	ch := w.list[id]
	if ch == nil {
		ch = make(chan interface{}, 1)
		w.list[id] = ch
	} else {
		panic(fmt.Errorf("dupicate id %x", id))
	}
	return ch
}

func (w *waitList) Trigger(id uint64, x interface{}) {
	w.mu.Lock()
	ch := w.list[id]
	delete(w.list, id)
	w.mu.Unlock()

	if ch != nil {
		ch <- x
		close(ch)
	}
}

func (w *waitList) IsRegistered(id uint64) bool {
	w.mu.Lock()
	_, ok := w.list[id]
	w.mu.Unlock()
	return ok
}

type waitWithResponse struct {
	ch <-chan interface{}
}

// NewWaitWithResponse returns Wait with response.
//
// (etcd pkg.wait.NewWithResponse)
func NewWaitWithResponse(ch <-chan interface{}) Wait {
	return &waitWithResponse{ch: ch}
}

func (w *waitWithResponse) Register(id uint64) <-chan interface{} { return w.ch }
func (w *waitWithResponse) Trigger(id uint64, x interface{})      {}
func (w *waitWithResponse) IsRegistered(id uint64) bool {
	panic("waitWithResponse.IsRegistered() shouldn't be called")
}
