package testutil

import (
	"runtime"
	"testing"
)

// FatalStack helps to fatal the test and print out the stacks of all running goroutines.
//
// (etcd pkg.testutil.FatalStack)
func FatalStack(t *testing.T, s string) {
	stackTrace := make([]byte, 8*1024)
	n := runtime.Stack(stackTrace, true)
	t.Error(string(stackTrace[:n]))
	t.Fatalf(s)
}
