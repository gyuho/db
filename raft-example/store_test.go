package main

import (
	"context"
	"testing"
)

func Test_store(t *testing.T) {
	propc := make(chan<- string)
	recvc := make(<-chan *string)
	errc := make(chan error)

	s := NewStore(propc, recvc, errc)
	defer s.Stop()

	s.Propose(context.TODO(), "foo", "bar")

	val, ok := s.Get("foo")
	if !ok {
		t.Fatal("ok expected true, got false")
	}
	if val != "bar" {
		t.Fatalf("value expected %q, got %q", "bar", val)
	}
}
