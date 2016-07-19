package crcutil

import (
	"hash/crc32"
	"reflect"
	"testing"
)

// TestHash32 tests that Hash32 provided by this package can take an initial
// crc and behaves exactly the same as the standard one in the following calls.
func TestHash32(t *testing.T) {
	stdHash := crc32.New(crc32.IEEETable)
	if _, err := stdHash.Write([]byte("test")); err != nil {
		t.Fatal(err)
	}
	// create a new hash with stdHash.Sum32() as initial crc
	crcHash := New(stdHash.Sum32(), crc32.IEEETable)

	stdHashSize := stdHash.Size()
	crcHashSize := crcHash.Size()
	if stdHashSize != crcHashSize {
		t.Fatalf("%d != %d", stdHashSize, crcHashSize)
	}

	stdHashBlockSize := stdHash.BlockSize()
	crcHashBlockSize := crcHash.BlockSize()
	if stdHashBlockSize != crcHashBlockSize {
		t.Fatalf("%d != %d", stdHashBlockSize, crcHashBlockSize)
	}

	stdHashSum32 := stdHash.Sum32()
	crcHashSum32 := crcHash.Sum32()
	if stdHashSum32 != crcHashSum32 {
		t.Fatalf("%d != %d", stdHashSum32, crcHashSum32)
	}

	stdHashSum := stdHash.Sum(make([]byte, 32))
	crcHashSum := crcHash.Sum(make([]byte, 32))
	if !reflect.DeepEqual(stdHashSum, crcHashSum) {
		t.Fatalf("sum = %v, want %v", crcHashSum, stdHashSum)
	}

	// write something
	if _, err := stdHash.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if _, err := crcHash.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	stdHashSum32 = stdHash.Sum32()
	crcHashSum32 = crcHash.Sum32()
	if stdHashSum32 != crcHashSum32 {
		t.Fatalf("%d != %d", stdHashSum32, crcHashSum32)
	}

	// reset
	stdHash.Reset()
	crcHash.Reset()
	stdHashSum32 = stdHash.Sum32()
	crcHashSum32 = crcHash.Sum32()
	if stdHashSum32 != crcHashSum32 {
		t.Fatalf("%d != %d", stdHashSum32, crcHashSum32)
	}
}
