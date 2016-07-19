package raftwal

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/gyuho/db/raft/raftpb"
)

func benchmarkWriteEntry(b *testing.B, size, batch int) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	w, err := Create(dir, []byte("metadata"))
	if err != nil {
		b.Fatal(err)
	}

	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte(i)
	}
	entry := &raftpb.Entry{Data: data}

	cnt := 0
	b.ResetTimer()
	b.SetBytes(int64(entry.Size()))
	for i := 0; i < b.N; i++ {
		w.Lock()
		if err := w.UnsafeEncodeEntry(entry); err != nil {
			b.Fatal(err)
		}

		cnt++
		if cnt > batch {
			w.UnsafeFdatasync()
			cnt = 0
		}
		w.Unlock()
	}
}

func BenchmarkWrite100EntryWithoutBatch(b *testing.B) { benchmarkWriteEntry(b, 100, 0) }
func BenchmarkWrite100EntryBatch10(b *testing.B)      { benchmarkWriteEntry(b, 100, 10) }
func BenchmarkWrite100EntryBatch100(b *testing.B)     { benchmarkWriteEntry(b, 100, 100) }
func BenchmarkWrite100EntryBatch500(b *testing.B)     { benchmarkWriteEntry(b, 100, 500) }
func BenchmarkWrite100EntryBatch1000(b *testing.B)    { benchmarkWriteEntry(b, 100, 1000) }

func BenchmarkWrite1000EntryWithoutBatch(b *testing.B) { benchmarkWriteEntry(b, 1000, 0) }
func BenchmarkWrite1000EntryBatch10(b *testing.B)      { benchmarkWriteEntry(b, 1000, 10) }
func BenchmarkWrite1000EntryBatch100(b *testing.B)     { benchmarkWriteEntry(b, 1000, 100) }
func BenchmarkWrite1000EntryBatch500(b *testing.B)     { benchmarkWriteEntry(b, 1000, 500) }
func BenchmarkWrite1000EntryBatch1000(b *testing.B)    { benchmarkWriteEntry(b, 1000, 1000) }
