package lmdb

import (
	"os"
	"time"
)

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
