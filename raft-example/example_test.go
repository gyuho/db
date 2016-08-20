// raft-example shows how to use raft package.
package main

import (
	"io/ioutil"
	"os"
	"syscall"

	"github.com/gyuho/db/pkg/osutil"
	"github.com/gyuho/db/pkg/xlog"
)

func Example() {
	xlog.SetGlobalMaxLogLevel(xlog.INFO)

	dir, err := ioutil.TempDir(os.TempDir(), "example.data")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	cfg := config{
		id:               1,
		clientURL:        "http://localhost:2379",
		advertisePeerURL: "http://localhost:2380",

		peerIDs:  []uint64{1},
		peerURLs: []string{"http://localhost:2380"},

		dir: dir,
	}
	rnd := startRaftNode(cfg)

	osutil.RegisterInterruptHandler(rnd.stop)
	osutil.WaitForInterruptSignals(syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go rnd.startClientHandler()

	<-rnd.donec
	logger.Println("done!")
}

/*
curl -L http://localhost:2379/foo -XPUT -d bar
curl -L http://localhost:2379/foo
*/
