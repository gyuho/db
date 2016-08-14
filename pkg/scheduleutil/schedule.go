package scheduleutil

import "time"

// WaitGoSchedule sleeps momentarily so that other goroutines can process.
//
// (etcd rafthttp.waitSchedule)
func WaitGoSchedule() { time.Sleep(1 * time.Millisecond) }

// WaitSchedule briefly sleeps in order to invoke the go scheduler.
//
// (etcd pkg.testutil.WaitSchedule)
func WaitSchedule() { time.Sleep(10 * time.Millisecond) }
