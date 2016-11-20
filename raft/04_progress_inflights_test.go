package raft

import (
	"reflect"
	"testing"
)

// (etcd raft.TestInflightsAdd)
func Test_inflights_add_no_rotating(t *testing.T) {
	ins := newInflights(10)

	// add flights
	for i := 0; i < 5; i++ {
		ins.add(uint64(i))
	}

	want1 := &inflights{
		//               ↓------------
		buffer:     []uint64{0, 1, 2, 3, 4, 0, 0, 0, 0, 0},
		bufferSize: 10,

		bufferStart: 0,
		bufferCount: 5,
	}
	if !reflect.DeepEqual(ins, want1) {
		t.Fatalf("expected %+v, got %+v", want1, ins)
	}

	// add flights
	for i := 5; i < 10; i++ {
		ins.add(uint64(i))
	}

	want2 := &inflights{
		//               ↓---------------------------
		buffer:     []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		bufferSize: 10,

		bufferStart: 0,
		bufferCount: 10,
	}
	if !reflect.DeepEqual(ins, want2) {
		t.Fatalf("expected %+v, got %+v", want2, ins)
	}
}

// (etcd raft.TestInflightsAdd)
func Test_inflights_add_rotating(t *testing.T) {
	ins := newInflights(10)
	ins.bufferStart = 5

	// add flights
	for i := 0; i < 5; i++ {
		ins.add(uint64(i * 100))
	}

	want1 := &inflights{
		//                              ↓--------------------
		buffer:     []uint64{0, 0, 0, 0, 0, 0, 100, 200, 300, 400},
		bufferSize: 10,

		bufferStart: 5,
		bufferCount: 5,
	}
	if !reflect.DeepEqual(ins, want1) {
		t.Fatalf("expected %+v, got %+v", want1, ins)
	}

	// add flights, trigger rotate
	for i := 5; i < 10; i++ {
		ins.add(uint64(i * 100))
	}

	want2 := &inflights{
		//               ↓----------------------  ↓--------------------
		buffer:     []uint64{500, 600, 700, 800, 900, 0, 100, 200, 300, 400},
		bufferSize: 10,

		bufferStart: 5,
		bufferCount: 10,
	}
	if !reflect.DeepEqual(ins, want2) {
		t.Fatalf("expected %+v, got %+v", want2, ins)
	}
}

// (etcd raft.TestInflightFreeTo)
func Test_inflights_freeTo(t *testing.T) {
	ins := newInflights(10)
	for i := 0; i < 10; i++ { // no rotation
		ins.add(uint64(i * 100))
	}

	// free inflights smaller than or equal to 400
	ins.freeTo(400)

	want1 := &inflights{
		//                                       ↓---------------------
		buffer:     []uint64{0, 100, 200, 300, 400, 500, 600, 700, 800, 900},
		bufferSize: 10,

		bufferStart: 5, // after freeTo 400
		bufferCount: 5,
	}
	if !reflect.DeepEqual(ins, want1) {
		t.Fatalf("expected %+v, got %+v", want1, ins)
	}

	// free inflights smaller than or equal to 800
	ins.freeTo(800)

	want2 := &inflights{
		//                                                           ↓
		buffer:     []uint64{0, 100, 200, 300, 400, 500, 600, 700, 800, 900},
		bufferSize: 10,

		bufferStart: 9, // after freeTo 800
		bufferCount: 1,
	}
	if !reflect.DeepEqual(ins, want2) {
		t.Fatalf("expected %+v, got %+v", want2, ins)
	}

	// rotating
	for i := 10; i < 15; i++ {
		ins.add(uint64(i * 100))
	}
	want3 := &inflights{
		//               ----------------------------                      ↓
		buffer:     []uint64{1000, 1100, 1200, 1300, 1400, 500, 600, 700, 800, 900},
		bufferSize: 10,

		bufferStart: 9,
		bufferCount: 6,
	}
	if !reflect.DeepEqual(ins, want3) {
		t.Fatalf("expected %+v, got %+v", want3, ins)
	}

	// free inflights smaller than or equal to 1200
	ins.freeTo(1200)

	want4 := &inflights{
		//                                 ↓--------
		buffer:     []uint64{1000, 1100, 1200, 1300, 1400, 500, 600, 700, 800, 900},
		bufferSize: 10,

		bufferStart: 3, // after freeto
		bufferCount: 2,
	}
	if !reflect.DeepEqual(ins, want4) {
		t.Fatalf("expected %+v, got %+v", want4, ins)
	}

	// doesn't affect anything
	// ins.freeTo(600)

	// free inflights smaller than or equal to 1400
	ins.freeTo(1400)

	want5 := &inflights{
		//                 ↓
		buffer:     []uint64{1000, 1100, 1200, 1300, 1400, 500, 600, 700, 800, 900},
		bufferSize: 10,

		bufferStart: 0, // after freeto
		bufferCount: 0,
	}
	if !reflect.DeepEqual(ins, want5) {
		t.Fatalf("expected %+v, got %+v", want5, ins)
	}
}

// (etcd raft.TestInflightFreeFirstOne)
func Test_inflights_freeFirstOne(t *testing.T) {
	ins := newInflights(10)
	for i := 100; i < 110; i++ {
		ins.add(uint64(i))
	}

	want1 := &inflights{
		//               ↓----------------------------------------------
		buffer:     []uint64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109},
		bufferSize: 10,

		bufferStart: 0,
		bufferCount: 10,
	}
	if !reflect.DeepEqual(ins, want1) {
		t.Fatalf("expected %+v, got %+v", want1, ins)
	}

	ins.freeFirstOne()

	want2 := &inflights{
		//                    ↓------------------------------------------
		buffer:     []uint64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109},
		bufferSize: 10,

		bufferStart: 1,
		bufferCount: 9,
	}
	if !reflect.DeepEqual(ins, want2) {
		t.Fatalf("expected %+v, got %+v", want2, ins)
	}
}
