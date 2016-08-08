package raftsnap

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/gyuho/db/raft/raftpb"
)

// (etcd snap.checkSuffix)
func checkSuffix(names []string) []string {
	snaps := []string{}
	for i := range names {
		if strings.HasSuffix(names[i], snapshotFileSuffix) {
			snaps = append(snaps, names[i])
		} else {
			// If we find a file which is not a snapshot then check if it's
			// a vaild file. If not throw out a warning.
			if _, ok := validFiles[names[i]]; !ok {
				logger.Warningf("skipped unexpected non snapshot file %v", names[i])
			}
		}
	}
	return snaps
}

// (etcd snap.Snapshotter.snapNames)
func getSnapNames(dir string) ([]string, error) {
	d, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer d.Close()

	names, err := d.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	snaps := checkSuffix(names)
	if len(snaps) == 0 {
		return nil, ErrNoSnapshot
	}

	sort.Sort(sort.Reverse(sort.StringSlice(snaps)))
	return snaps, nil
}

// (etcd snap.renameBroken)
func renameBroken(path string) {
	brokenPath := path + ".broken"
	if err := os.Rename(path, brokenPath); err != nil {
		logger.Warningf("cannot rename broken snapshot file %v to %v: %v", path, brokenPath, err)
	}
}

func getSnapFileName(snapshot *raftpb.Snapshot) string {
	return fmt.Sprintf("%016x-%016x%s", snapshot.Metadata.Term, snapshot.Metadata.Index, snapshotFileSuffix)
}
