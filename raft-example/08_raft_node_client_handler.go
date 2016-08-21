package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/gyuho/db/pkg/types"
)

func (rnd *raftNode) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
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
		rnd.ds.propose(context.TODO(), kv)

		// not yet committed, so subsetquent GET may return stale data
		fmt.Fprintf(rw, "proposing %+v\n", kv)

		// rw.WriteHeader(http.StatusNoContent)

	case "POST": // TODO: implement config change
	case "DELETE": // TODO: implement config change

	case "GET":
		if val, ok := rnd.ds.get(key); ok {
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
	go func() {
		err := <-rnd.ds.errc
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
		Handler: rnd,
	}
	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}
