package lmdb

import (
	"os"
	"time"
)

// data file format version
//
// (bolt.version)
const version = 2

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

const (
	// DefaultMaxBatchSize is the maximum batch size for DB operations.
	//
	// (bolt.DefaultMaxBatchSize)
	DefaultMaxBatchSize int = 1000

	// DefaultMaxBatchDelay for DB operations.
	//
	// (bolt.DefaultMaxBatchDelay)
	DefaultMaxBatchDelay = 10 * time.Millisecond

	// DefaultAllocBytesN for DB operations.
	//
	// (bolt.DefaultAllocSize)
	DefaultAllocBytesN = 16 * 1024 * 1024 // 16 MB
)

// (bolt.defaultPageSize)
var defaultPageSize = os.Getpagesize()
