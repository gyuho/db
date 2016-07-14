package idutil

import (
	"testing"
	"time"
)

func TestNewGenerator(t *testing.T) {
	g := NewGenerator(0x12, time.Unix(0, 0).Add(0x3456*time.Millisecond))
	id := g.Next()
	wid := uint64(0x12000000345601)
	if id != wid {
		t.Fatalf("id expected %x, got %x", wid, id)
	}
}

func TestNewGeneratorUnique(t *testing.T) {
	g := NewGenerator(0, time.Time{})
	id := g.Next()

	// after restart
	gRestart := NewGenerator(0, time.Now())
	if idRestart := gRestart.Next(); id == idRestart {
		t.Fatalf("expected %x != %x", id, idRestart)
	}

	// different server generates different ID
	gDifferent := NewGenerator(1, time.Now())
	if idDifferent := gDifferent.Next(); id == idDifferent {
		t.Fatalf("expected %x != %x", id, idDifferent)
	}
}

func TestNext(t *testing.T) {
	g := NewGenerator(0x12, time.Unix(0, 0).Add(0x3456*time.Millisecond))
	wid := uint64(0x12000000345601)
	for i := 0; i < 1000; i++ {
		id := g.Next()
		if id != wid+uint64(i) {
			t.Fatalf("id expected %x, got %x", wid+uint64(i), id)
		}
	}
}
