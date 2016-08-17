package scheduleutil

import (
	"testing"
	"time"
)

// (etcd pkg.wait.TestWaitTime)
func Test_NewWaitWithDeadline(t *testing.T) {
	wt := NewWaitWithDeadline()

	t1 := time.Now()
	ch1 := wt.Wait(t1)
	wt.Trigger(time.Unix(0, t1.UnixNano()+1))
	select {
	case <-ch1:
	default:
		t.Fatalf("cannot receive from ch as expected")
	}

	t2 := time.Now()
	ch2 := wt.Wait(t2)
	wt.Trigger(t2)
	select {
	case <-ch2:
		t.Fatal("unexpected to receive from ch2")
	default:
	}

	wt.Trigger((time.Unix(0, t2.UnixNano()+1)))
	select {
	case <-ch2:
	default:
		t.Fatal("cannot receive from ch2 as expected")
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
