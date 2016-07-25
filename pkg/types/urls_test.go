package types

import (
	"reflect"
	"testing"

	"github.com/coreos/etcd/pkg/testutil"
)

func TestNewURLs(t *testing.T) {
	tests := []struct {
		strs  []string
		wurls URLs
	}{
		{
			[]string{"http://127.0.0.1:2379"},
			testutil.MustNewURLs(t, []string{"http://127.0.0.1:2379"}),
		},
		{ // trim space
			[]string{"   http://127.0.0.1:2379    "},
			testutil.MustNewURLs(t, []string{"http://127.0.0.1:2379"}),
		},
		{ // sort
			[]string{
				"http://127.0.0.2:2379",
				"http://127.0.0.1:2379",
			},
			testutil.MustNewURLs(t, []string{
				"http://127.0.0.1:2379",
				"http://127.0.0.2:2379",
			}),
		},
	}

	for i, tt := range tests {
		urls, _ := NewURLs(tt.strs)
		if !reflect.DeepEqual(urls, tt.wurls) {
			t.Fatalf("#%d: urls expected %+v, got %+v", i, tt.wurls, urls)
		}
	}
}

func TestURLsString(t *testing.T) {
	tests := []struct {
		us   URLs
		wstr string
	}{
		{
			URLs{},
			"",
		},
		{
			testutil.MustNewURLs(t, []string{"http://127.0.0.1:2379"}),
			"http://127.0.0.1:2379",
		},
		{
			testutil.MustNewURLs(t, []string{
				"http://127.0.0.1:2379",
				"http://127.0.0.2:2379",
			}),
			"http://127.0.0.1:2379,http://127.0.0.2:2379",
		},
		{
			testutil.MustNewURLs(t, []string{
				"http://127.0.0.2:2379",
				"http://127.0.0.1:2379",
			}),
			"http://127.0.0.2:2379,http://127.0.0.1:2379",
		},
	}
	for i, tt := range tests {
		g := tt.us.String()
		if g != tt.wstr {
			t.Fatalf("#%d: string expected %q, got %q", i, tt.wstr, g)
		}
	}
}

func TestURLsSort(t *testing.T) {
	g := testutil.MustNewURLs(t, []string{
		"http://127.0.0.4:2379",
		"http://127.0.0.2:2379",
		"http://127.0.0.1:2379",
		"http://127.0.0.3:2379",
	})
	w := testutil.MustNewURLs(t, []string{
		"http://127.0.0.1:2379",
		"http://127.0.0.2:2379",
		"http://127.0.0.3:2379",
		"http://127.0.0.4:2379",
	})
	gurls := URLs(g)
	gurls.Sort()

	if !reflect.DeepEqual(g, w) {
		t.Fatalf("URLs expected %+v, got %+v", w, g)
	}
}

func TestURLsStringSlice(t *testing.T) {
	tests := []struct {
		us   URLs
		wstr []string
	}{
		{
			URLs{},
			[]string{},
		},
		{
			testutil.MustNewURLs(t, []string{"http://127.0.0.1:2379"}),
			[]string{"http://127.0.0.1:2379"},
		},
		{
			testutil.MustNewURLs(t, []string{
				"http://127.0.0.1:2379",
				"http://127.0.0.2:2379",
			}),
			[]string{"http://127.0.0.1:2379", "http://127.0.0.2:2379"},
		},
		{
			testutil.MustNewURLs(t, []string{
				"http://127.0.0.2:2379",
				"http://127.0.0.1:2379",
			}),
			[]string{"http://127.0.0.2:2379", "http://127.0.0.1:2379"},
		},
	}
	for i, tt := range tests {
		g := tt.us.StringSlice()
		if !reflect.DeepEqual(g, tt.wstr) {
			t.Fatalf("#%d: string slice = %+v, want %+v", i, g, tt.wstr)
		}
	}
}

func TestNewURLsFail(t *testing.T) {
	tests := [][]string{
		{}, // no urls given
		{"://127.0.0.1:2379"},          // missing protocol scheme
		{"mailto://127.0.0.1:2379"},    // unsupported scheme
		{"http://127.0.0.1"},           // not conform to host:port
		{"http://127.0.0.1:2379/path"}, // contain a path
	}

	for i, tt := range tests {
		_, err := NewURLs(tt)
		if err == nil {
			t.Fatalf("#%d: expected err, got nil", i)
		}
	}
}
