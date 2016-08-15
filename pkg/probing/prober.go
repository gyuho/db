package probing

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("probing: id not found")
	ErrExist    = errors.New("probing: id already exists")
)

// Prober defines probing operation.
type Prober interface {
	AddHTTP(id string, interval time.Duration, endpoints []string) error

	Remove(id string) error
	RemoveAll()

	Reset(id string) error

	Status(id string) (Status, error)
}

// (probing.prober)
type probe struct {
	mu         sync.Mutex
	idToStatus map[string]*status
	tr         http.RoundTripper
}

func NewProber(tr http.RoundTripper) Prober {
	p := &probe{
		idToStatus: make(map[string]*status),
		tr:         tr,
	}

	if p.tr == nil {
		p.tr = http.DefaultTransport
	}
	return p
}

func (p *probe) AddHTTP(id string, interval time.Duration, endpoints []string) error {
	p.mu.Lock()
	if _, ok := p.idToStatus[id]; ok {
		p.mu.Unlock()
		return ErrExist
	}

	s := newStatus()
	p.idToStatus[id] = s
	p.mu.Unlock()

	ticker := time.NewTicker(interval)

	go func() {
		pinnedIdx := 0
		for {
			select {
			case <-ticker.C:
				start := time.Now()
				req, err := http.NewRequest("GET", endpoints[pinnedIdx], nil)
				if err != nil {
					panic(err)
				}

				resp, err := p.tr.RoundTrip(req)
				if err != nil {
					s.recordFailure(err)
					pinnedIdx = (pinnedIdx + 1) % len(endpoints)
					continue
				}

				var health Health
				err = json.NewDecoder(resp.Body).Decode(&health)
				resp.Body.Close()
				if err != nil || !health.OK {
					s.recordFailure(err)
					pinnedIdx = (pinnedIdx + 1) % len(endpoints)
					continue
				}

				s.record(time.Since(start), health.RequestedTime)

			case <-s.stopc:
				ticker.Stop()
				return
			}
		}
	}()

	return nil
}

func (p *probe) Remove(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	s, ok := p.idToStatus[id]
	if !ok {
		return ErrNotFound
	}
	close(s.stopc)
	delete(p.idToStatus, id)

	return nil

}

func (p *probe) RemoveAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, s := range p.idToStatus {
		close(s.stopc)
	}
	p.idToStatus = make(map[string]*status)
}

func (p *probe) Reset(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	s, ok := p.idToStatus[id]
	if !ok {
		return ErrNotFound
	}
	s.reset()
	return nil
}

func (p *probe) Status(id string) (Status, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	s, ok := p.idToStatus[id]
	if !ok {
		return nil, ErrNotFound
	}

	return s, nil
}
