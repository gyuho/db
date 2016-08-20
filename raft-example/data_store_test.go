package main

import (
	"context"
	"testing"
	"time"
)

func Test_dataStore(t *testing.T) {
	ds := newDataStore(make(chan []byte), make(chan []byte), make(chan error))
	defer ds.stop()

	donec := make(chan struct{})
	go func() {
		defer close(donec)

		bts := <-ds.propc

		// assume this is agreed by consensus
		ds.commitc <- bts
	}()
	ds.propose(context.TODO(), keyValue{"foo", "bar"})
	<-donec

	time.Sleep(10 * time.Millisecond)

	val, ok := ds.get("foo")
	if !ok {
		t.Fatal("ok expected true, got false")
	}
	if val != "bar" {
		t.Fatalf("value expected %q, got %q", "bar", val)
	}

	close(ds.errc)
	if err := <-ds.errc; err != nil {
		t.Fatal(err)
	}
}
