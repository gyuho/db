package idutil

import (
	"math"
	"sync"
	"time"
)

// Generator generates unique uint64 id based on member ID, timestamp, and counter.
//
// (etcd idutil.Generator)
type Generator struct {
	mu sync.Mutex

	// high order 2 bytes with member ID
	prefix uint64

	// lower order 6 bytes
	// 5 bytes are for timestamps
	// 1 byte is for counter
	suffix uint64
}

// NewGenerator returns a new Generator.
//
// (etcd idutil.NewGenerator)
func NewGenerator(memberID uint16, now time.Time) *Generator {
	prefix := uint64(memberID) << (8 * 6) // first(high) 2 bytes

	msec := uint64(now.UnixNano()) / uint64(time.Millisecond)
	suffix := lowByteBit(msec, 8*5)
	suffix = suffix << 8 // ???

	return &Generator{
		prefix: prefix,
		suffix: suffix,
	}
}

func lowByteBit(x uint64, n uint) uint64 {
	return x & (math.MaxUint64 >> (8*8 - n)) // ???
}

// Next generates the next unique ID.
func (g *Generator) Next() uint64 {
	g.mu.Lock()
	g.suffix++
	id := g.prefix | lowByteBit(g.suffix, 8*6)
	g.mu.Unlock()

	return id
}