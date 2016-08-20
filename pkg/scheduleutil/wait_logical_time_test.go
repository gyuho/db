package scheduleutil

import (
	"testing"
	"time"
)

// (etcd pkg.wait.TestWaitTime)
func Test_NewWaitLogicalTime(t *testing.T) {
	wt := NewWaitLogicalTime()
	ch1 := wt.Wait(1)
	wt.Trigger(2)

	select {
	case <-ch1:
	default:
		t.Fatalf("cannot receive from ch as expected")
	}

	ch2 := wt.Wait(4)
	wt.Trigger(3)

	select {
	case <-ch2:
		t.Fatal("unexpected to receive from ch2")
	default:
	}

	wt.Trigger(4)
	select {
	case <-ch2:
	default:
		t.Fatal("cannot receive from ch2 as expected")
	}

	select {
	// wait on a triggered deadline
	case <-wt.Wait(4):
	default:
		t.Fatalf("unexpected blocking when wait on triggered deadline")
	}
}

// (etcd pkg.wait.TestWaitTestStress)
func Test_NewWaitLogicalTime_Stress(t *testing.T) {
	wt := NewWaitLogicalTime()

	var chs []<-chan struct{}
	for i := 0; i < 10000; i++ {
		chs = append(chs, wt.Wait(uint64(i)))
	}

	wt.Trigger(10000 + 1)

	for _, ch := range chs {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("cannot receive from ch as expected")
		}
	}
}
