package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gyuho/db/pkg/netutil"
	"github.com/gyuho/db/pkg/xlog"
)

func Test_basic_single_node(t *testing.T) {
	ports, err := netutil.GetFreeTCPPorts(2)
	if err != nil {
		panic(err)
	}

	xlog.SetGlobalMaxLogLevel(xlog.INFO)

	dir, err := ioutil.TempDir(os.TempDir(), "example.data")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cfg := config{
		id:               1,
		clientURL:        fmt.Sprintf("http://localhost:%d", ports[0]),
		advertisePeerURL: fmt.Sprintf("http://localhost:%d", ports[1]),

		peerIDs:  []uint64{1},
		peerURLs: []string{fmt.Sprintf("http://localhost:%d", ports[1])},

		dir: dir,
	}
	rnd := startRaftNode(cfg)

	go rnd.startClientHandler()

	donec := make(chan struct{})
	go func() {
		defer close(donec)

		time.Sleep(300 * time.Millisecond)
		func() {
			req, err := http.NewRequest("PUT", cfg.clientURL+"/foo", strings.NewReader("bar"))
			if err != nil {
				t.Fatal(err)
			}
			tr := http.DefaultTransport
			resp, err := tr.RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			bts, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Println("PUT response:", string(bts))
		}()

		time.Sleep(300 * time.Millisecond)
		func() {
			req, err := http.NewRequest("GET", cfg.clientURL+"/foo", nil)
			if err != nil {
				t.Fatal(err)
			}
			tr := http.DefaultTransport
			resp, err := tr.RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			bts, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Println("GET response:", string(bts))
		}()
	}()
	<-donec
	rnd.stop()

	<-rnd.donec
	fmt.Println("test done!")
}

/*
curl -L http://localhost:2379/foo -XPUT -d bar
curl -L http://localhost:2379/foo
*/
