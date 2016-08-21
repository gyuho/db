package main

import (
	"github.com/gyuho/db/pkg/fileutil"
	"github.com/gyuho/db/raftwal"
	"github.com/gyuho/db/raftwal/raftwalpb"
)

func (rnd *raftNode) openWAL() *raftwal.WAL {
	if !fileutil.DirHasFiles(rnd.walDir) {
		if err := fileutil.MkdirAll(rnd.walDir); err != nil {
			panic(err)
		}

		w, err := raftwal.Create(rnd.walDir, nil)
		if err != nil {
			panic(err)
		}
		w.Close()
	}

	w, err := raftwal.OpenWALWrite(rnd.walDir, raftwalpb.Snapshot{})
	if err != nil {
		panic(err)
	}
	return w
}

func (rnd *raftNode) replayWAL() *raftwal.WAL {
	w := rnd.openWAL()

	// TODO: repair
	_, hardstate, ents, err := w.ReadAll()
	if err != nil {
		panic(err)
	}

	rnd.storageMemory.Append(ents...)

	if len(ents) == 0 {
		rnd.commitc <- nil // to inform that commit channel is current
	} else {
		rnd.lastIndex = ents[len(ents)-1].Index
	}

	rnd.storageMemory.SetHardState(hardstate)
	return w
}
