package scheduleutil

import (
	"fmt"
	"sync"
	"time"
)

// Action defines an action with name and parameters.
//
// (etcd pkg.testutil.Action)
type Action struct {
	Name       string
	Parameters []interface{}
}

// Recorder defines action recorder interface.
//
// (etcd pkg.testutil.Recorder)
type Recorder interface {
	// Record publishes an Action (e.g., function call) which will
	// be reflected by Wait() or Chan().
	Record(a Action)

	// Wait waits until at least n Actions are available or returns with error.
	Wait(n int) ([]Action, error)

	// Action returns immediately available Actions.
	Action() []Action

	// Chan returns the channel for actions published by Record.
	Chan() <-chan Action
}

// recorderBuffered appends all Actions to a slice.
//
// (etcd pkg.testutil.RecorderBuffered)
type recorderBuffered struct {
	mu      sync.Mutex
	actions []Action
}

// NewRecorderBuffered returns a new Recorder with buffered actions.
func NewRecorderBuffered() Recorder {
	return &recorderBuffered{}
}

func (r *recorderBuffered) Record(a Action) {
	r.mu.Lock()
	r.actions = append(r.actions, a)
	r.mu.Unlock()
}

func (r *recorderBuffered) Action() []Action {
	r.mu.Lock()
	cpy := make([]Action, len(r.actions))
	copy(cpy, r.actions)
	r.mu.Unlock()
	return cpy
}

func (r *recorderBuffered) Wait(n int) (acts []Action, err error) {
	// sleeps in order to invoke the go scheduler.
	// TODO: improve this when we are able to know the schedule or status of target go-routine.
	time.Sleep(10 * time.Millisecond)

	acts = r.Action()
	if len(acts) < n {
		err = newLenErr(n, len(acts))
	}
	return acts, err
}

func (r *recorderBuffered) Chan() <-chan Action {
	ch := make(chan Action)
	go func() {
		acts := r.Action()
		for i := range acts {
			ch <- acts[i]
		}
		close(ch)
	}()
	return ch
}

///////////////////////////////////////////

//
// to satisfy Wait interface
//

func (r *recorderBuffered) Register(id uint64) <-chan interface{} {
	r.Record(Action{Name: "Register"})
	return nil
}
func (r *recorderBuffered) Trigger(id uint64, x interface{}) {
	r.Record(Action{Name: "Trigger"})
}

func (r *recorderBuffered) IsRegistered(id uint64) bool {
	panic("recorderBuffered.IsRegistered() shouldn't be called")
}

///////////////////////////////////////////

// recorderStream writes all Actions to an unbuffered channel
//
// (etcd pkg.testutil.recorderStream)
type recorderStream struct {
	ch chan Action
}

// NewRecorderStream returns a new Recorder with stream ch.
//
// (etcd pkg.testutil.NewRecorderStream)
func NewRecorderStream() Recorder {
	return &recorderStream{ch: make(chan Action)}
}

func (r *recorderStream) Record(a Action) {
	r.ch <- a
}

func (r *recorderStream) Action() (acts []Action) {
	for {
		select {
		case act := <-r.ch:
			acts = append(acts, act)
		default:
			return acts
		}
	}
}

func (r *recorderStream) Chan() <-chan Action {
	return r.ch
}

func (r *recorderStream) Wait(n int) ([]Action, error) {
	acts := make([]Action, n)
	timeoutC := time.After(5 * time.Second)
	for i := 0; i < n; i++ {
		select {
		case acts[i] = <-r.ch:
		case <-timeoutC:
			acts = acts[:i]
			return acts, newLenErr(n, i)
		}
	}

	// extra wait to catch any Action spew
	select {
	case act := <-r.ch:
		acts = append(acts, act)
	case <-time.After(10 * time.Millisecond):
	}
	return acts, nil
}

func newLenErr(expected int, actual int) error {
	return fmt.Errorf("len(actions) = %d, expected >= %d", actual, expected)
}
