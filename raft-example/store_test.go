package main

import (
	"context"
	"testing"
	"time"
)

func Test_store(t *testing.T) {
	propc := make(chan []byte)
	commitc := make(chan []byte)
	errc := make(chan error)

	s := newStore(propc, commitc, errc)
	defer s.stop()

	donec := make(chan struct{})
	go func() {
		defer close(donec)

		bts := <-propc

		// assume this is agreed by consensus
		commitc <- bts
	}()
	s.propose(context.TODO(), keyValue{"foo", "bar"})
	<-donec

	time.Sleep(10 * time.Millisecond)

	val, ok := s.get("foo")
	if !ok {
		t.Fatal("ok expected true, got false")
	}
	if val != "bar" {
		t.Fatalf("value expected %q, got %q", "bar", val)
	}

	close(errc)
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
}
