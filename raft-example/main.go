package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"syscall"

	"github.com/gyuho/db/pkg/osutil"
	"github.com/gyuho/db/pkg/xlog"
)

func main() {
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		start(1)
	}()
	go func() {
		defer wg.Done()
		start(2)
	}()
	go func() {
		defer wg.Done()
		start(3)
	}()

	wg.Wait()
}

func init() {
	xlog.SetGlobalMaxLogLevel(xlog.INFO)
}

func start(id uint64) {
	dir, err := ioutil.TempDir(os.TempDir(), fmt.Sprintf("example.data.%d", id))
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	cfg := config{
		id:               id,
		clientURL:        fmt.Sprintf("http://localhost:%d2379", id),
		advertisePeerURL: fmt.Sprintf("http://localhost:%d2380", id),

		peerIDs:  []uint64{1, 2, 3},
		peerURLs: []string{"http://localhost:12380", "http://localhost:22380", "http://localhost:32380"},

		dir: dir,
	}
	rnd := startRaftNode(cfg)

	osutil.RegisterInterruptHandler(rnd.stop)
	osutil.WaitForInterruptSignals(syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go rnd.startClientHandler()

	<-rnd.donec
	logger.Println("done!", id)
}
