package main

import "testing"

func Test_basic_1_node(t *testing.T) {
	propc, commitc := make(chan []byte), make(chan []byte)
	errc := make(chan error)

	cfg := config{
		id:  1,
		url: "http://127.0.0.1:2380",

		peerIDs:  []uint64{1},
		peerURLs: []string{"http://127.0.0.1:2380"},

		dir: "testdata",
	}
	rnd := newRaftNode(cfg)
	startRaftHandler(":2379", rnd.pro)
}
