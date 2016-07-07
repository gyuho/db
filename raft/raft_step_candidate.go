package raft

// (etcd raft.raft.poll)
func (rnd *raftNode) receiveVotes(fromID uint64, voted bool) int {
	if voted {
		raftLogger.Infof("%x received vote from %x at term %d", rnd.id, fromID, rnd.term)
	} else {
		raftLogger.Infof("%x received vote-rejection from %x at term %d", rnd.id, fromID, rnd.term)
	}

	if _, ok := rnd.votedFrom[fromID]; !ok {
		rnd.votedFrom[fromID] = voted
	} else { // ???
		raftLogger.Panicf("%x received duplicate votes from %x (voted %v)", rnd.id, fromID, voted)
	}

	grantedN := 0
	for _, voted := range rnd.votedFrom {
		if voted {
			grantedN++
		}
	}

	return grantedN
}

// (etcd raft.raft.becomeCandidate)
func (rnd *raftNode) becomeCandidate() {

}
