package raftsnap

import (
	"errors"
	"hash/crc32"
)

const snapshotFileSuffix = ".snap"

var (
	ErrNoSnapshot    = errors.New("raftsnap: no available snapshot")
	ErrEmptySnapshot = errors.New("raftsnap: empty snapshot")
	ErrCRCMismatch   = errors.New("raftsnap: crc mismatch")

	crcTable = crc32.MakeTable(crc32.Castagnoli)

	// A map of valid files that can be present in the snap folder.
	validFiles = map[string]bool{
		"db": true,
	}
)

// Snapshotter contains directory where snapshot file exists.
//
// (etcd etcd.snap.Snapshotter)
type Snapshotter struct {
	dir string
}

// New returns a new Snapshotter.
//
// (etcd etcd.snap.New)
func New(dir string) *Snapshotter {
	return &Snapshotter{
		dir: dir,
	}
}
