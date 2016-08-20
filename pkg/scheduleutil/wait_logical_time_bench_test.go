package scheduleutil

import "testing"

// (etcd pkg.wait.BenchmarkWaitTime)
func Benchmark_NewWaitLogicalTime(b *testing.B) {
	wt := NewWaitLogicalTime()
	for i := 0; i < b.N; i++ {
		wt.Wait(1)
	}
}

// (etcd pkg.wait.BenchmarkTriggerAnd10KWaitTime)
func Benchmark_NewWaitLogicalTime_Trigger(b *testing.B) {
	for i := 0; i < b.N; i++ {
		wt := NewWaitLogicalTime()
		for j := 0; j < 10000; j++ {
			wt.Wait(uint64(j))
		}
		wt.Trigger(10000 + 1)
	}
}
