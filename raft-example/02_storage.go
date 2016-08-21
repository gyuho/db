package main

import (
	"github.com/gyuho/db/raft/raftpb"
	"github.com/gyuho/db/raftsnap"
	"github.com/gyuho/db/raftwal"
	"github.com/gyuho/db/raftwal/raftwalpb"
)

// (etcd etcdserver.Storage)
type Storage interface {
	// Save function saves ents and state to the underlying stable storage.
	// Save MUST block until st and ents are on stable storage.
	Save(st raftpb.HardState, ents []raftpb.Entry) error
	// raftwal.WAL.Save

	// SaveSnap function saves snapshot to the underlying stable storage.
	SaveSnap(snap raftpb.Snapshot) error
	// raftsnap.Snapshotter.SaveSnap

	// DBFilePath returns the file path of database snapshot saved with given id.
	DBFilePath(id uint64) (string, error)
	// raftsnap.Snapshotter.DBFilePath

	// Close closes the Storage and performs finalization.
	Close() error
	// raftwal.WAL.Close
}

type storage struct {
	*raftwal.WAL
	*raftsnap.Snapshotter
}

func newStorage(w *raftwal.WAL, s *raftsnap.Snapshotter) Storage {
	return &storage{w, s}
}

func (s *storage) SaveSnap(snap raftpb.Snapshot) error {
	walSnap := raftwalpb.Snapshot{
		Index: snap.Metadata.Index,
		Term:  snap.Metadata.Term,
	}
	if err := s.WAL.UnsafeEncodeSnapshotAndFdatasync(&walSnap); err != nil {
		return err
	}

	if err := s.Snapshotter.SaveSnap(snap); err != nil {
		return err
	}

	return s.WAL.ReleaseLocks(snap.Metadata.Index)
}
