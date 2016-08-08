package raftsnap

import (
	"hash/crc32"
	"os"
	"path/filepath"

	"github.com/gyuho/db/pkg/fileutil"
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftsnap/raftsnappb"
)

// (etcd snap.Snapshotter.save)
func (s *Snapshotter) save(snapshot *raftpb.Snapshot) error {
	b, err := snapshot.Marshal()
	if err != nil {
		return err
	}

	crc := crc32.Update(0, crcTable, b)
	sSnap := raftsnappb.Snapshot{CRC: crc, Data: b}
	d, err := sSnap.Marshal()
	if err != nil {
		return err
	}

	fname := getSnapFileName(snapshot)
	err = fileutil.WriteSync(filepath.Join(s.dir, fname), d, fileutil.PrivateFileMode)
	if err != nil {
		if e := os.Remove(filepath.Join(s.dir, fname)); e != nil {
			logger.Errorf("failed to remove broken snapshot file %s", filepath.Join(s.dir, fname))
		}
	}
	return err
}

// Save saves raftpb.Snapshot to Snapshotter.
//
// (etcd snap.Snapshotter.SaveSnap)
func (s *Snapshotter) Save(snapshot raftpb.Snapshot) error {
	if raftpb.IsEmptySnapshot(snapshot) {
		return nil
	}
	return s.save(&snapshot)
}
