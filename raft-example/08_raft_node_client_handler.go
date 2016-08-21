package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/gyuho/db/pkg/types"
)

type clientHandler struct {
	ds *dataStore
}

func (hd *clientHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	key := req.RequestURI

	switch req.Method {
	case "PUT":
		val, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Warningf("failed to read on PUT (%v)", err)
			http.Error(rw, "PUT failure", http.StatusBadRequest)
			return
		}
		kv := keyValue{Key: key, Val: string(val)}

		logger.Printf("proposing %+v", kv)
		hd.ds.propose(context.TODO(), kv)

		// not yet committed, so subsetquent GET may return stale data
		fmt.Fprintf(rw, "proposing %+v\n", kv)

		// rw.WriteHeader(http.StatusNoContent)

	case "POST": // TODO
	case "DELETE": // TODO

	case "GET":
		if val, ok := hd.ds.get(key); ok {
			fmt.Fprintln(rw, val) // rw.Write([]byte(val))
		} else {
			fmt.Fprintf(rw, "%q does not exist\n", key)
		}

	default:
		rw.Header().Set("Allow", "PUT")
		rw.Header().Add("Allow", "GET")
		rw.Header().Add("Allow", "POST")
		rw.Header().Add("Allow", "DELETE")
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (rnd *raftNode) startClientHandler() {
	ds := newDataStore(rnd.propc, rnd.commitc)
	go func() {
		err := <-ds.errc
		if err != nil {
			panic(err)
		}
	}()

	_, port, err := net.SplitHostPort(rnd.clientURL.Host)
	if err != nil {
		panic(err)
	}

	logger.Printf("startClientHandler %s with %q", types.ID(rnd.id), rnd.clientURL.String())
	srv := http.Server{
		Addr:    ":" + port,
		Handler: &clientHandler{ds: ds},
	}
	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}
