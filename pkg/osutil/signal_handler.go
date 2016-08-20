package osutil

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// InterruptHandler is a function that is called on receiving signals (SIGTERM, SIGINT, SIGQUIT).
// SIGQUIT prints stack trace.
//
// (etcd pkg.osutil.InterruptHandler)
type InterruptHandler func()

var (
	mu                sync.Mutex
	interruptHandlers []InterruptHandler
)

// RegisterInterruptHandler registers InterruptHandler.
//
// (etcd pkg.osutil.RegisterInterruptHandler)
func RegisterInterruptHandler(s InterruptHandler) {
	mu.Lock()
	interruptHandlers = append(interruptHandlers, s)
	mu.Unlock()
}

// WaitForInterruptSignals waits for signals and call handlers.
//
// (etcd pkg.osutil.HandleInterrupts)
func WaitForInterruptSignals(sigs ...os.Signal) {
	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, sigs...)

	go func() {
		sig := <-notifier

		mu.Lock()
		copied := make([]InterruptHandler, len(interruptHandlers))
		copy(copied, interruptHandlers)
		mu.Unlock()

		logger.Warningf("received %v signal, shutting down...", sig)
		for _, ihFunc := range copied {
			ihFunc()
		}

		// stop receiving signals
		signal.Stop(notifier)

		pid := syscall.Getpid()
		// exit directly if it is the "init" process, since the kernel will not help to kill pid 1.
		if pid == 1 {
			os.Exit(0)
		}

		logger.Warningf("sending syscall.Kill %s to PID %d", sig, pid)
		syscall.Kill(pid, sig.(syscall.Signal))
		logger.Warningf("sent syscall.Kill %s to PID %d", sig, pid)
	}()
}
