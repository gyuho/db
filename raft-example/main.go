// raft-example shows how to use raft package.
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gyuho/db/pkg/osutil"
	"github.com/gyuho/db/pkg/xlog"
)

func main() {
	nodeNum := 3

	var (
		dataDirs   []string
		peerIDs    []uint64
		clientURLs []string
		peerURLs   []string
	)
	for i := 1; i <= nodeNum; i++ {
		dir, err := ioutil.TempDir(os.TempDir(), fmt.Sprintf("test.data.%d", i))
		if err != nil {
			panic(err)
		}
		dataDirs = append(dataDirs, dir)

		peerIDs = append(peerIDs, uint64(i))

		clientURLs = append(clientURLs, fmt.Sprintf("http://localhost:%d2379", i))
		peerURLs = append(peerURLs, fmt.Sprintf("http://localhost:%d2380", i))
	}

	var wg sync.WaitGroup
	wg.Add(nodeNum)

	for i := range peerIDs {
		go func(idx int) {
			defer wg.Done()

			start(peerIDs[idx], clientURLs[idx], peerURLs[idx], peerIDs, peerURLs)
		}(i)
	}

	/*
	   curl -L http://localhost:12379/foo -XPUT -d bar
	   curl -L http://localhost:12379/foo

	   curl -L http://localhost:22379/foo -XPUT -d bar
	   curl -L http://localhost:22379/foo

	   curl -L http://localhost:32379/foo -XPUT -d bar
	   curl -L http://localhost:32379/foo
	*/
	time.Sleep(time.Second)
	donec := make(chan struct{})
	go func() {
		defer close(donec)

		time.Sleep(time.Second)
		func() {
			req, err := http.NewRequest("PUT", "http://localhost:12379/foo", strings.NewReader("bar"))
			if err != nil {
				panic(err)
			}
			tr := http.DefaultTransport
			resp, err := tr.RoundTrip(req)
			if err != nil {
				panic(err)
			}
			defer resp.Body.Close()

			bts, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}
			fmt.Println("PUT response:", string(bts))
		}()

		time.Sleep(time.Second)
		func() {
			req, err := http.NewRequest("GET", "http://localhost:32379/foo", nil) // different node
			if err != nil {
				panic(err)
			}
			tr := http.DefaultTransport
			resp, err := tr.RoundTrip(req)
			if err != nil {
				panic(err)
			}
			defer resp.Body.Close()

			bts, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}
			fmt.Println("GET response:", string(bts))
		}()
	}()
	<-donec

	wg.Wait()
}

func init() {
	xlog.SetGlobalMaxLogLevel(xlog.INFO)
}

func start(id uint64, clientURL, advertisePeerURL string, peerIDs []uint64, peerURLs []string) {
	dir, err := ioutil.TempDir(os.TempDir(), fmt.Sprintf("example.data.%d", id))
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	cfg := config{
		id:               id,
		clientURL:        clientURL,
		advertisePeerURL: advertisePeerURL,

		peerIDs:  peerIDs,
		peerURLs: peerURLs,

		dir: dir,
	}
	rnd := startRaftNode(cfg)

	osutil.RegisterInterruptHandler(rnd.stop)
	osutil.WaitForInterruptSignals(syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go rnd.startClientHandler()

	<-rnd.donec
	logger.Println("done!", id)
}
