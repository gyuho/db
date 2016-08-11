package raftsnap

import (
	"hash/crc32"
	"io/ioutil"
	"path/filepath"

	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftsnap/raftsnappb"
)

// Read reads the snapshot named by snapname and returns raftpb.Snapshot.
//
// (etcd snap.Read)
func Read(snapFileName string) (*raftpb.Snapshot, error) {
	b, err := ioutil.ReadFile(snapFileName)
	if err != nil {
		logger.Errorf("cannot read file %v: %v", snapFileName, err)
		return nil, err
	}

	if len(b) == 0 {
		logger.Errorf("unexpected empty snapshot")
		return nil, ErrEmptySnapshot
	}

	var sSnap raftsnappb.Snapshot
	if err = sSnap.Unmarshal(b); err != nil {
		logger.Errorf("corrupted snapshot file %v: %v", snapFileName, err)
		return nil, err
	}

	if len(sSnap.Data) == 0 || sSnap.CRC == 0 {
		logger.Errorf("unexpected empty snapshot")
		return nil, ErrEmptySnapshot
	}

	crc := crc32.Update(0, crcTable, sSnap.Data)
	if crc != sSnap.CRC {
		logger.Errorf("corrupted snapshot file %v: crc mismatch", snapFileName)
		return nil, ErrCRCMismatch
	}

	var rSnap raftpb.Snapshot
	if err = rSnap.Unmarshal(sSnap.Data); err != nil {
		logger.Errorf("corrupted snapshot file %v: %v", snapFileName, err)
		return nil, err
	}
	return &rSnap, nil
}

// (etcd snap.loadSnap)
func load(dir, name string) (*raftpb.Snapshot, error) {
	fpath := filepath.Join(dir, name)
	snap, err := Read(fpath)
	if err != nil {
		renameBroken(fpath)
	}
	return snap, err
}

// Load loads snapshot.
//
// (etcd snap.Snapshotter.Load)
func (s *Snapshotter) Load() (*raftpb.Snapshot, error) {
	names, err := getSnapNames(s.dir)
	if err != nil {
		return nil, err
	}

	var snap *raftpb.Snapshot
	for _, name := range names {
		if snap, err = load(s.dir, name); err == nil {
			break
		}
	}
	if err != nil {
		return nil, ErrNoSnapshot
	}

	return snap, nil
}
