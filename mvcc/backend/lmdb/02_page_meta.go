package lmdb

import (
	"fmt"
	"os"
	"sort"
	"unsafe"
)

// maxMmapStepBytesN is the largest step that can be taken
// when remapping the mmap.
//
// (bolt.maxMmapStep)
const maxMmapStepBytesN = 1 << 30 // 1 GB

// (bolt_amd64.go maxMapSize)
const maxMmapSize = 0xFFFFFFFFFFFF // 256 TB

// maxAllocSize is the maximum size of pointer array.
//
// (bolt_amd64.go maxAllocSize)
const maxAllocSize = 0x7FFFFFFF // 2 GB

// maxBranchLeafSize is the maximum size of pointer arrays of leaf, branch page element.
const maxBranchLeafSize = 0x7FFFFFF // 134 MB

// pgid is the page ID.
//
// (bolt.pgid)
type pgid uint64

type pgids []pgid

func (s pgids) Len() int           { return len(s) }
func (s pgids) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s pgids) Less(i, j int) bool { return s[i] < s[j] }

// merge returns the sorted union of a and b.
//
// (bolt.pgids.merge)
func (a pgids) merge(b pgids) (merged pgids) {
	if len(a) == 0 {
		merged = b
		return
	}
	if len(b) == 0 {
		merged = a
		return
	}

	merged = make(pgids, 0, len(a)+len(b))

	// lead with lower starting value
	lead, follow := a, b
	if b[0] < a[0] {
		lead, follow = b, a
	}

	for len(lead) > 0 {
		// find largest slice of lead that is smaller than follow[0]
		//
		// Search uses binary search to find and return the smallest index i
		// in [0, n) at which f(i) is true; Search returns the first true index.
		// If there is no such index, Search returns n.
		idx := sort.Search(len(lead), func(i int) bool {
			return lead[i] > follow[0]
		})
		merged = append(merged, lead[:idx]...)

		// all elements in 'lead' are smaller than follow[0]
		if idx >= len(lead) {
			break
		}

		lead, follow = follow, lead[idx:]
	}

	// append what's left in follow
	merged = append(merged, follow...)
	return
}

// bucket represents the on-file representation of a Bucket.
// bucket is stored as value that corresponds to the bucket key.
// If the bucket is small enough, then its root page can be stored
// inline in the value, after the bucket header (TODO: ???).
// In case of inline buckets, the root will be 0.
//
// (bolt.bucket)
type bucket struct {
	// root is the page ID of the bucket's root-level page.
	root pgid

	// sequence monotonically increases, used by NextSequence.
	sequence uint64
}

// (bolt.page)
type page struct {
	id       pgid
	flags    uint16
	count    uint16
	overflow uint32

	// A uintptr is an integer, not a reference.
	// Even if a uintptr holds the address of some object,
	// the garbage collector will not update that uintptr's value
	// if the object moves, nor will that uintptr keep the object
	// from being reclaimed.
	ptr uintptr

	// unsafe.Pointer is a pointer, while uintptr is just a number.
	// Both represent memory address.
}

type pages []*page

func (s pages) Len() int           { return len(s) }
func (s pages) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s pages) Less(i, j int) bool { return s[i].id < s[j].id }

/*
& (AND)

Let f be &

	1. f(a, b) = f(b, a)
	2. f(a, a) = a
	3. f(a, b) ≤ max(a, b)


∨ (OR)

Let f be ∨

	1. f(a, b) = f(b, a)
	2. f(a, a) = a
	3. f(a, b) ≥ max(a, b)
*/
const (
	branchPageFlag   = 0x01
	leafPageFlag     = 0x02
	metaPageFlag     = 0x04
	freelistPageFlag = 0x10

	bucketLeafFlag = 0x01
)

// (bolt.page.typ)
func (p *page) String() string {
	switch {
	case (p.flags & branchPageFlag) != 0:
		return "branch"
	case (p.flags & leafPageFlag) != 0:
		return "leaf"
	case (p.flags & metaPageFlag) != 0:
		return "meta"
	case (p.flags & freelistPageFlag) != 0:
		return "freelist"
	}
	return fmt.Sprintf("unknown<0x%02X>", p.flags)
}

// hexdump writes n bytes of the page to stderr in hexadecimal.
//
// (bolt.page.hexdump)
func (p *page) hexdump(n int) {
	buf := (*[maxAllocSize]byte)(unsafe.Pointer(p))[:n]
	fmt.Fprintf(os.Stderr, "%x\n", buf)
}

// branchPageElement represents a node on a branch page.
//
// (bolt.branchPageElement)
type branchPageElement struct {
	pos   uint32
	ksize uint32
	pgid  pgid
}

func (n *branchPageElement) key() []byte {
	buf := (*[maxAllocSize]byte)(unsafe.Pointer(n))
	return (*[maxAllocSize]byte)(unsafe.Pointer(&buf[n.pos]))[:n.ksize]
}

// leafPageElement represents a node on a leaf page.
//
// (bolt.leafPageElement)
type leafPageElement struct {
	flags uint32
	pos   uint32
	ksize uint32
	vsize uint32
}

func (n *leafPageElement) key() []byte {
	buf := (*[maxAllocSize]byte)(unsafe.Pointer(n))
	return (*[maxAllocSize]byte)(unsafe.Pointer(&buf[n.pos]))[:n.ksize:n.ksize]
}

func (n *leafPageElement) value() []byte {
	buf := (*[maxAllocSize]byte)(unsafe.Pointer(n))
	return (*[maxAllocSize]byte)(unsafe.Pointer(&buf[n.pos+n.ksize]))[:n.vsize:n.vsize]
}

// marker to indicate that the file is lmdb.
//
// (bolt.magic)
const marker uint32 = 0XED0CDAED

// data file format version
//
// (bolt.version)
const version = 2

// (bolt.meta)
type meta struct {
	marker   uint32
	version  uint32
	pageSize uint32
	flags    uint32
	root     bucket
	freelist pgid
	pgid     pgid
	txid     txid
	checksum uint64
}

// meta returns the pointer value to the metadata section of the page.
func (p *page) meta() *meta {
	return (*meta)(unsafe.Pointer(&p.ptr))
}

const (
	pageHeaderSize      = int(unsafe.Offsetof(((*page)(nil)).ptr))
	branchElementSize   = int(unsafe.Sizeof(branchPageElement{}))
	leafPageElementSize = int(unsafe.Sizeof(leafPageElement{}))
)
