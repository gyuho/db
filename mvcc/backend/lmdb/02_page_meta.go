package lmdb

import "fmt"

// marker to indicate that the file is lmdb.
//
// (bolt.magic)
const magic uint32 = 0XED0CDAED

// maxMmapStepBytesN is the largest step that can be taken
// when remapping the mmap (1 GB).
//
// (bolt.maxMmapStep)
const maxMmapStepBytesN = 1 << 30

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

// bucket represents the on-file representation of a Bucket.
// bucket is stored as value that corresponds to the bucket key.
// If the bucket is small enough, then its root page can be stored
// inline in the value, after the bucket header (???).
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
	ptr      uintptr
}

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
	return fmt.Sprintf("unknown<0x%02x>", p.flags)
}
