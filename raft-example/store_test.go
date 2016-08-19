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

	s := NewStore(propc, recvc, errc)
	defer s.Stop()

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		str := <-propc
		recvc <- &str
	}()
	s.Propose(context.TODO(), "foo", "bar")
	<-donec

	time.Sleep(10 * time.Millisecond)

	val, ok := s.Get("foo")
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
