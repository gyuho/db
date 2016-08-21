package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gyuho/db/pkg/xlog"
)

func Test_basic_single_node(t *testing.T) {
	xlog.SetGlobalMaxLogLevel(xlog.INFO)

	dir, err := ioutil.TempDir(os.TempDir(), "example.data")
	if err != nil {
		t.Fatal(err)
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

	go rnd.startClientHandler()

	donec := make(chan struct{})
	go func() {
		defer close(donec)

		time.Sleep(time.Second)
		func() {
			req, err := http.NewRequest("PUT", "http://localhost:2379/foo", strings.NewReader("bar"))
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

		time.Sleep(time.Second)
		func() {
			req, err := http.NewRequest("GET", "http://localhost:2379/foo", nil)
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
