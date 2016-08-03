package scheduleutil

import (
	"testing"
	"time"
)

// (etcd pkg.wait.BenchmarkWaitTime)
func Benchmark_NewWaitWithDeadline(b *testing.B) {
	t := time.Now()
	wt := NewWaitWithDeadline()
	for i := 0; i < b.N; i++ {
		wt.Wait(t)
	}
}

// (etcd pkg.wait.BenchmarkTriggerAnd10KWaitTime)
func Benchmark_NewWaitWithDeadline_Trigger(b *testing.B) {
	for i := 0; i < b.N; i++ {
		t := time.Now()
		wt := NewWaitWithDeadline()
		for j := 0; j < 10000; j++ {
			wt.Wait(t)
		}
		wt.Trigger(time.Now())
	}
}
