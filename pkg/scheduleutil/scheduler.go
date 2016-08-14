package scheduleutil

import (
	"context"
	"sync"
	"time"
)

// WaitSchedule briefly sleeps in order to invoke the go scheduler.
//
// (etcd pkg.testutil.WaitSchedule)
func WaitSchedule() { time.Sleep(10 * time.Millisecond) }

// Job is a function to be run with context.
//
// (etcd pkg.schedule.Job)
type Job func(context.Context)

// Scheduler defines scheduler interface.
//
// (etcd pkg.schedule.Scheduler)
type Scheduler interface {
	// Schedule asks the scheduler to schedule a job defined by the given func.
	// Schedule to a stopped scheduler might panic.
	Schedule(j Job)

	// Pending returns number of pending jobs.
	Pending() int

	// Scheduled returns the number of scheduled jobs (excluding pending jobs).
	Scheduled() int

	// Finished returns the number of finished jobs.
	Finished() int

	// WaitFinish waits until at least n job are finished and all pending jobs are finished.
	WaitFinish(n int)

	// Stop stops the whole Scheduler.
	Stop()
}

type fifo struct {
	mu sync.Mutex

	resume    chan struct{}
	scheduled int
	finished  int
	pendings  []Job

	ctx    context.Context
	cancel context.CancelFunc

	finishCond *sync.Cond
	donec      chan struct{}
}

// NewSchedulerFIFO returns a Scheduler that schedules jobs in FIFO order.
//
// (etcd pkg.schedule.NewFIFOScheduler)
func NewSchedulerFIFO() Scheduler {
	f := &fifo{
		resume: make(chan struct{}, 1),
		donec:  make(chan struct{}, 1),
	}
	f.ctx, f.cancel = context.WithCancel(context.Background())
	f.finishCond = sync.NewCond(&f.mu)

	go f.run()

	return f
}

// Schedule schedules a job that will be ran in FIFO order sequentially.
func (f *fifo) Schedule(j Job) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.cancel == nil {
		panic("schedule: schedule to stopped scheduler")
	}

	if len(f.pendings) == 0 {
		select {
		case f.resume <- struct{}{}:
		default:
		}
	}
	f.pendings = append(f.pendings, j)

	return
}

func (f *fifo) Pending() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.pendings)
}

func (f *fifo) Scheduled() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.scheduled
}

func (f *fifo) Finished() int {
	f.finishCond.L.Lock()
	defer f.finishCond.L.Unlock()
	return f.finished
}

func (f *fifo) WaitFinish(n int) {
	f.finishCond.L.Lock()
	for f.finished < n || len(f.pendings) != 0 {
		// Wait atomically unlocks c.L and suspends execution
		// of the calling goroutine. After later resuming execution,
		// Wait locks c.L before returning. Unlike in other systems,
		// Wait cannot return unless awoken by Broadcast or Signal.
		f.finishCond.Wait()
	}
	f.finishCond.L.Unlock()
}

// Stop stops the scheduler and cancels all pending jobs.
func (f *fifo) Stop() {
	f.mu.Lock()
	f.cancel()
	f.cancel = nil
	f.mu.Unlock()

	<-f.donec
}

func (f *fifo) run() {
	defer func() {
		close(f.donec)
		close(f.resume)
	}()

	for {
		var todo Job
		f.mu.Lock()
		if len(f.pendings) != 0 {
			f.scheduled++
			todo = f.pendings[0]
		}
		f.mu.Unlock()

		if todo == nil {
			select {
			case <-f.resume:
			case <-f.ctx.Done():
				f.mu.Lock()
				pendings := f.pendings
				f.pendings = nil
				f.mu.Unlock()

				for _, todo := range pendings {
					todo(f.ctx)
				}
				return
			}
			continue
		}

		todo(f.ctx)

		f.finishCond.L.Lock()
		f.finished++
		f.pendings = f.pendings[1:]
		f.finishCond.Broadcast()
		f.finishCond.L.Unlock()
	}
}
