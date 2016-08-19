package main

import (
	"context"
	"testing"
	"time"
)

func Test_store(t *testing.T) {
	propc := make(chan string)
	recvc := make(chan *string)
	errc := make(chan error)

	s := newStore(propc, recvc, errc)
	defer s.stop()

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		str := <-propc
		recvc <- &str
	}()
	s.propose(context.TODO(), "foo", "bar")
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
