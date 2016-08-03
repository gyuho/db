package scheduleutil

import (
	"fmt"
	"testing"
	"time"
)

// (etcd pkg.scheduleutil.TestNew)
func Test_NewWait(t *testing.T) {
	const eid = 1

	wt := NewWait()
	ch := wt.Register(eid)

	wt.Trigger(eid, "foo")

	v := <-ch
	if g, w := fmt.Sprintf("%v (%T)", v, v), "foo (string)"; g != w {
		t.Fatalf("<-ch = %v, want %v", g, w)
	}

	if g := <-ch; g != nil {
		t.Fatalf("unexpected non-nil value: %v (%T)", g, g)
	}
}

// (etcd pkg.scheduleutil.TestRegisterDupPanic)
func Test_NewWait_RegisterDupPanic(t *testing.T) {
	const eid = 1

	wt := NewWait()
	ch1 := wt.Register(eid)

	pchan := make(chan struct{}, 1)

	func() {
		defer func() {
			if r := recover(); r != nil {
				pchan <- struct{}{}
			}
		}()
		wt.Register(eid)
	}()

	select {
	case <-pchan:
	case <-time.After(1 * time.Second):
		t.Fatalf("failed to receive panic")
	}

	wt.Trigger(eid, "foo")
	<-ch1
}

// (etcd pkg.scheduleutil.TestTriggerDupSuppression)
func Test_NewWait_TriggerDupSuppression(t *testing.T) {
	const eid = 1

	wt := NewWait()
	ch := wt.Register(eid)

	wt.Trigger(eid, "foo")
	wt.Trigger(eid, "bar")

	v := <-ch
	if g, w := fmt.Sprintf("%v (%T)", v, v), "foo (string)"; g != w {
		t.Fatalf("<-ch = %v, want %v", g, w)
	}

	if g := <-ch; g != nil {
		t.Fatalf("unexpected non-nil value: %v (%T)", g, g)
	}
}

// (etcd pkg.scheduleutil.TestIsRegistered)
func Test_NewWait_IsRegistered(t *testing.T) {
	wt := NewWait()

	wt.Register(0)
	wt.Register(1)
	wt.Register(2)

	for i := uint64(0); i < 3; i++ {
		if !wt.IsRegistered(i) {
			t.Fatalf("event ID %d isn't registered", i)
		}
	}

	if wt.IsRegistered(4) {
		t.Fatalf("event ID 4 shouldn't be registered")
	}

	wt.Trigger(0, "foo")

	if wt.IsRegistered(0) {
		t.Fatalf("event ID 0 is already triggered, shouldn't be registered")
	}
}
