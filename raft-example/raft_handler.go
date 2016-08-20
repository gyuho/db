package main

import (
	"context"
	"io/ioutil"
	"net/http"
)

type raftHandler struct {
	ds *dataStore
}

func (hd *raftHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "PUT":
		key := req.RequestURI
		val, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Warningf("failed to read on PUT (%v)", err)
			http.Error(rw, "PUT failure", http.StatusBadRequest)
			return
		}
		kv := keyValue{Key: key, Val: string(val)}
		hd.ds.propose(context.TODO(), kv)
		logger.Printf("proposed %+v", kv)

		// not yet committed, so subsetquent GET may return stale data
		rw.WriteHeader(http.StatusNoContent)

	case "POST": // TODO
	case "DELETE": // TODO

	case "GET":
		key := req.RequestURI
		if val, ok := hd.ds.get(key); ok {
			rw.Write([]byte(val))
			return
		}
		http.Error(rw, "GET failure", http.StatusNotFound)

	default:
		rw.Header().Set("Allow", "PUT")
		rw.Header().Add("Allow", "GET")
		rw.Header().Add("Allow", "POST")
		rw.Header().Add("Allow", "DELETE")
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func startRaftHandler(addr string, propc, commitc chan []byte, errc chan error) {
	ds := newDataStore(propc, commitc, errc)
	go func() {
		err := <-ds.errc
		if err != nil {
			logger.Panic(err)
		}
	}()

	logger.Printf("startRaftHandler with %q", addr)
	srv := http.Server{
		Addr: addr,
		Handler: &raftHandler{
			ds: ds,
		},
	}
	if err := srv.ListenAndServe(); err != nil {
		logger.Panic(err)
	}
}
