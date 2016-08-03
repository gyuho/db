package netutil

import (
	"net/http"
	"sync"
)

// PauseableHandler is http.Handler that can be paused.
//
// (etcd pkg.testutil.PauseableHandler)
type PauseableHandler struct {
	Next http.Handler

	mu     sync.Mutex
	paused bool
}

func (ph *PauseableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ph.mu.Lock()
	paused := ph.paused
	ph.mu.Unlock()
	if !paused {
		ph.Next.ServeHTTP(w, r)
	} else {
		hj, ok := w.(http.Hijacker)
		if !ok {
			panic("webserver doesn't support hijacking")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			panic(err.Error())
		}
		conn.Close()
	}
}

// Pause pauses.
func (ph *PauseableHandler) Pause() {
	ph.mu.Lock()
	ph.paused = true
	ph.mu.Unlock()
}

// Resume resumes.
func (ph *PauseableHandler) Resume() {
	ph.mu.Lock()
	ph.paused = false
	ph.mu.Unlock()
}
