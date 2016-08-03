package scheduleutil

// WaitRecorder wraps Wait and Recorder interfaces.
//
// (etcd pkg.mock.mockwait.WaitRecorder)
type WaitRecorder struct {
	Wait
	Recorder
}

type waitRecorder struct {
	recorderBuffered
}

// NewMockWaitRecorder returns a new recorder with Wait.
//
// (etcd pkg.mock.mockwait.NewRecorder)
func NewMockWaitRecorder() *WaitRecorder {
	wr := &waitRecorder{}
	return &WaitRecorder{Wait: wr, Recorder: wr}
}
