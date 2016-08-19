package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"sync"
)

type kv struct {
	Key string
	Val string
}

type store struct {
	propc chan<- string
	recvc <-chan *string
	errc  chan error

	stopc chan struct{}
	donec chan struct{}

	mu   sync.RWMutex
	data map[string]string
}

func newStore(propc chan<- string, recvc <-chan *string, errc chan error) *store {
	s := &store{
		propc: propc,
		recvc: recvc,
		errc:  errc,

		stopc: make(chan struct{}),
		donec: make(chan struct{}),

		data: make(map[string]string),
	}
	go s.read()
	return s
}

func (s *store) get(key string) (string, bool) {
	s.mu.RLock()
	v, ok := s.data[key]
	s.mu.RUnlock()
	return v, ok
}

func (s *store) propose(ctx context.Context, k, v string) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(kv{Key: k, Val: v}); err != nil {
		s.errc <- err
		return
	}
	data := string(buf.Bytes())

	for {
		select {
		case s.propc <- data:
			return

		case <-ctx.Done():
			s.errc <- ctx.Err()
			return

		case <-s.donec:
			return

		default:
		}
	}
}

func (s *store) stop() {
	close(s.stopc)
	<-s.donec
}

func (s *store) read() {
	for {
		select {
		case sp := <-s.recvc:
			if sp == nil {
				continue
			}
			var data kv
			if err := gob.NewDecoder(bytes.NewBufferString(*sp)).Decode(&data); err != nil {
				logger.Panic(err)
			}
			s.mu.Lock()
			s.data[data.Key] = data.Val
			s.mu.Unlock()

		case err := <-s.errc:
			logger.Panic(err)

		case <-s.stopc:
			close(s.donec)
			return
		}
	}
}
