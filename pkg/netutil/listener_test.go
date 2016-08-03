package netutil

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/gyuho/db/pkg/tlsutil"
)

// (etcd pkg.transport.TestNewListenerUnixSocket)
func Test_NewListenerUnix(t *testing.T) {
	l, err := NewListener("testsocket-address", "unix", nil)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
}

// (etcd pkg.transport.TestNewListenerTLSEmptyInfo)
func Test_NewListener_Empty_TLSInfo(t *testing.T) {
	if _, err := NewListener("127.0.0.1:0", "https", nil); err == nil {
		t.Fatal("err = nil, want not presented error")
	}
}

func testNewListenerTLSInfoAccept(t *testing.T, tlsInfo tlsutil.TLSInfo) {
	tlsServerCfg, err := tlsInfo.ServerConfig()
	if err != nil {
		t.Fatal(err)
	}

	ln, err := NewListener("127.0.0.1:0", "https", tlsServerCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go http.Get("https://" + ln.Addr().String())

	conn, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if _, ok := conn.(*tls.Conn); !ok {
		t.Fatal("failed to accept *tls.Conn")
	}
}
