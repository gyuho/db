package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"sync"
)

type keyValue struct {
	Key string
	Val string
}

type store struct {
	propc   chan<- []byte
	commitc <-chan []byte
	errc    chan error

	stopc chan struct{}
	donec chan struct{}

	mu   sync.RWMutex
	data map[string]string
}

func newStore(propc chan<- []byte, commitc <-chan []byte, errc chan error) *store {
	s := &store{
		propc:   propc,
		commitc: commitc,
		errc:    errc,

		stopc: make(chan struct{}),
		donec: make(chan struct{}),

		data: make(map[string]string),
	}
	go s.readCommit()
	return s
}

func (s *store) get(key string) (string, bool) {
	s.mu.RLock()
	v, ok := s.data[key]
	s.mu.RUnlock()
	return v, ok
}

func (s *store) propose(ctx context.Context, kv keyValue) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(kv); err != nil {
		s.errc <- err
		return
	}
	data := buf.Bytes()

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

func (s *store) readCommit() {
	for {
		select {
		case cc := <-s.commitc:
			if len(cc) == 0 {
				continue
			}
			var kv keyValue
			if err := gob.NewDecoder(bytes.NewBuffer(cc)).Decode(&kv); err != nil {
				logger.Panic(err)
			}
			s.mu.Lock()
			s.data[kv.Key] = kv.Val
			s.mu.Unlock()

		case err := <-s.errc:
			if err != nil {
				logger.Panic(err)
			}

		case <-s.stopc:
			close(s.donec)
			return

		case <-s.donec:
			return
		}
	}
}
