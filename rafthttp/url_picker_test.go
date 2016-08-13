package rafthttp

import (
	"net/url"
	"testing"

	"github.com/gyuho/db/pkg/types"
)

func Test_URLPicker_twice(t *testing.T) {
	umap := map[url.URL]bool{
		url.URL{Scheme: "http", Host: "127.0.0.1:2380"}: true,
		url.URL{Scheme: "http", Host: "127.0.0.1:7001"}: true,
	}
	picker := newURLPicker(types.MustNewURLs([]string{"http://127.0.0.1:2380", "http://127.0.0.1:7001"}))

	u := picker.pick()
	if !umap[u] {
		t.Fatalf("expected %+v, got none in %+v", u, umap)
	}

	u2 := picker.pick()
	if u != u2 {
		t.Fatalf("%+v != %+v", u, u2)
	}
}

func Test_URLPicker_update(t *testing.T) {
	picker := newURLPicker(types.MustNewURLs([]string{"http://127.0.0.1:2380", "http://127.0.0.1:7001"}))
	picker.update(types.MustNewURLs([]string{"http://localhost:2380", "http://localhost:7001"}))

	umap := map[url.URL]bool{
		url.URL{Scheme: "http", Host: "localhost:2380"}: true,
		url.URL{Scheme: "http", Host: "localhost:7001"}: true,
	}
	u := picker.pick()
	if !umap[u] {
		t.Fatalf("expected %+v, got none in %+v", u, umap)
	}
}

func Test_URLPicker_unreachable(t *testing.T) {
	picker := newURLPicker(types.MustNewURLs([]string{"http://127.0.0.1:2380", "http://127.0.0.1:7001"}))
	u := picker.pick()

	picker.unreachable(u)

	u2 := picker.pick()
	if u == u2 {
		t.Fatalf("%+v == %+v", u, u2)
	}
}
