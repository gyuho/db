package raft

import "github.com/gyuho/db/raft/raftpb"

func applyToStore(ents []raftpb.Entry)    {}
func sendMessages(msgs []raftpb.Message)  {}
func saveStateToDisk(st raftpb.HardState) {}
func saveToDisk(ents []raftpb.Entry)      {}

func ExampleNode() {
	config := &Config{}
	nd := StartNode(config, nil)
	defer nd.Stop()

	// last known hardState
	var prevHardState raftpb.HardState
	for {
		rd := <-nd.Ready()
		if !rd.HardStateToSave.Equal(prevHardState) {
			saveStateToDisk(rd.HardStateToSave)
			prevHardState = rd.HardStateToSave
		}

		saveToDisk(rd.EntriesToAppend)
		go applyToStore(rd.EntriesToApply)
		sendMessages(rd.MessagesToSend)
	}
}
