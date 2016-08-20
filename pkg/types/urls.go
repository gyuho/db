package types

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
)

// NewURL parses string URL.
func NewURL(txt string) (url.URL, error) {
	txt = strings.TrimSpace(txt)
	u, err := url.Parse(txt)
	if err != nil {
		return url.URL{}, err
	}

	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "unix" && u.Scheme != "unixs" {
		return url.URL{}, fmt.Errorf("URL scheme must be http, https, unix, or unixs: %q", txt)
	}

	if _, _, err := net.SplitHostPort(u.Host); err != nil {
		return url.URL{}, fmt.Errorf("URL address does not have the form host:port (%q, %v)", txt, err)
	}

	if u.Path != "" {
		return url.URL{}, fmt.Errorf("URL must not contain a path: %q", txt)
	}

	return *u, nil
}

// MustNewURL returns url.URL from string.
func MustNewURL(txt string) url.URL {
	url, err := NewURL(txt)
	if err != nil {
		panic(err)
	}
	return url
}

// URLs is a slice of 'url.URL'.
type URLs []url.URL

// NewURLs returns URLs for the given string slice.
func NewURLs(strs []string) (URLs, error) {
	all := make([]url.URL, len(strs))
	if len(all) == 0 {
		return nil, errors.New("no valid URLs given")
	}

	for i, txt := range strs {
		u, err := NewURL(txt)
		if err != nil {
			return nil, err
		}
		all[i] = u
	}

	us := URLs(all)
	us.Sort()

	return us, nil
}

// Sort sorts URLs.
func (us *URLs) Sort() {
	sort.Sort(us)
}

func (us URLs) Len() int           { return len(us) }
func (us URLs) Less(i, j int) bool { return us[i].String() < us[j].String() }
func (us URLs) Swap(i, j int)      { us[i], us[j] = us[j], us[i] }

// MustNewURLs returns URLs for the given string slice.
//
// (etcd rafthttp.mustNewURLPicker)
func MustNewURLs(strs []string) URLs {
	urls, err := NewURLs(strs)
	if err != nil {
		panic(err)
	}
	return urls
}

func (us URLs) String() string {
	return strings.Join(us.StringSlice(), ",")
}

// StringSlice converts URLs to string slice.
//
// (etcd pkg.types.URLs.StringSlice)
func (us URLs) StringSlice() []string {
	out := make([]string, len(us))
	for i := range us {
		out[i] = us[i].String()
	}
	return out
}
