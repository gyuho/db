package rafthttp

import (
	"net/url"
	"sync"

	"github.com/gyuho/db/pkg/types"
)

// urlPicker picks URL with pinned index.
//
// (etcd rafthttp.urlPicker)
type urlPicker struct {
	mu        sync.Mutex
	urls      types.URLs
	pickedIdx int
}

func newURLPicker(urls types.URLs) *urlPicker {
	return &urlPicker{urls: urls}
}

func (p *urlPicker) update(urls types.URLs) {
	p.mu.Lock()
	p.urls = urls
	p.pickedIdx = 0
	p.mu.Unlock()
}

func (p *urlPicker) pick() url.URL {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.urls[p.pickedIdx]
}

func (p *urlPicker) unreachable(u url.URL) {
	p.mu.Lock()
	if u == p.urls[p.pickedIdx] {
		p.pickedIdx = (p.pickedIdx + 1) % len(p.urls)
	}
	p.mu.Unlock()
}
