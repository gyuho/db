package scheduleutil

import (
	"testing"
	"time"
)

// (etcd pkg.wait.TestWaitTime)
func Test_NewWaitWithDeadline(t *testing.T) {
	wt := NewWaitWithDeadline()

	ch1 := wt.Wait(time.Now())
	t1 := time.Now()

	wt.Trigger(t1)
	select {
	case <-ch1:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("cannot receive from ch as expected")
	}

	ch2 := wt.Wait(time.Now())
	t2 := time.Now()

	wt.Trigger(t1)
	select {
	case <-ch2:
		t.Fatal("unexpected to receive from ch")
	case <-time.After(10 * time.Millisecond):
	}

	wt.Trigger(t2)
	select {
	case <-ch2:
	case <-time.After(10 * time.Millisecond):
		t.Fatal("cannot receive from ch as expected")
	}
}

// (etcd pkg.wait.TestWaitTestStress)
func Test_NewWaitWithDeadline_Stress(t *testing.T) {
	wt := NewWaitWithDeadline()

	var chs []<-chan struct{}
	for i := 0; i < 10000; i++ {
		chs = append(chs, wt.Wait(time.Now()))
		time.Sleep(time.Nanosecond)
	}

	wt.Trigger(time.Now())

	for _, ch := range chs {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("cannot receive from ch as expected")
		}
	}
}
