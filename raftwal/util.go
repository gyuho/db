package raftwal

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/gyuho/db/pkg/fileutil"
)

func getWALName(seq, index uint64) string {
	return fmt.Sprintf("%016x-%016x.wal", seq, index)
}

// (etcd wal.parseWalName)
func parseWALName(name string) (seq, index uint64, err error) {
	if filepath.Ext(name) != ".wal" {
		return 0, 0, fmt.Errorf("bad WAL name")
	}
	_, err = fmt.Sscanf(name, "%016x-%016x.wal", &seq, &index)
	return
}

// (etcd wal.checkWalNames)
func selectWALNames(names []string) []string {
	var wnames []string
	for _, name := range names {
		if _, _, err := parseWALName(name); err != nil {
			if filepath.Ext(name) != ".tmp" {
				// only complain about non-WAL temp files
				logger.Warningf("ignored %q (%v)", name, err)
			}
			continue
		}
		wnames = append(wnames, name)
	}
	return wnames
}

// readWALNames reads all the WAL files in the directory.
// And the results must be sorted.
//
// (etcd wal.readWALNames)
func readWALNames(dir string) ([]string, error) {
	names, err := fileutil.ReadDir(dir) // this reads and sorts
	if err != nil {
		return nil, err
	}

	wnames := selectWALNames(names)
	if len(wnames) == 0 {
		return nil, ErrFileNotFound
	}
	return wnames, nil
}

// areWALNamesSorted returns true if WAL names are correctly sorted.
// They should have been sorted based on sequence number
// (sequence number should increase continuously).
//
// (etcd wal.isValidSeq)
func areWALNamesSorted(names []string) bool {
	var lastSeq uint64
	for _, name := range names {
		curSeq, _, err := parseWALName(name)
		if err != nil {
			logger.Panicf("parseWALName(%q) should never fail (%v)", name, err)
		}

		if lastSeq != 0 && lastSeq != curSeq-1 {
			return false
		}
		lastSeq = curSeq
	}
	return true
}

// (etcd wal.closeAll)
func closeAll(rcs ...io.ReadCloser) error {
	for _, f := range rcs {
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}

// searchLastWALIndex finds the last slice index of WAL names
// whose Raft index section is equal to or smaller than the given index.
// It assumes that the given names are already sorted.
// It returns -1 if all indexes are greater than the given index.
//
// (etcd wal.searchIndex)
func searchLastWALIndex(ns []string, idx uint64) int {
	for i := len(ns) - 1; i >= 0; i-- {
		name := ns[i]
		_, curIndex, err := parseWALName(name)
		if err != nil {
			logger.Panicf("parseWALName(%q) failed (%v)", name, err)
		}
		if idx >= curIndex {
			return i
		}
	}
	return -1
}
