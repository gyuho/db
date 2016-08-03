package netutil

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"testing"

	"github.com/gyuho/db/pkg/tlsutil"
)

// (etcd pkg.transport.TestNewKeepAliveListener)
func Test_NewListenerWithKeepAlive(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("unexpected listen error: %v", err)
	}

	ln, err = NewListenerWithKeepAlive(ln, "http", nil)
	if err != nil {
		t.Fatalf("unexpected NewListenerWithKeepAlive error: %v", err)
	}

	go http.Get("http://" + ln.Addr().String())

	conn, err := ln.Accept()
	if err != nil {
		t.Fatalf("unexpected Accept error: %v", err)
	}
	conn.Close()
	ln.Close()

	ln, err = net.Listen("tcp", "127.0.0.1:0")

	// tls
	tmp, err := createTempFile([]byte("testdata"))
	if err != nil {
		t.Fatalf("unable to create tmpfile: %v", err)
	}
	defer os.Remove(tmp)

	tlsInfo := tlsutil.TLSInfo{CertFile: tmp, KeyFile: tmp}
	tlsInfo.ParseFunc = fakeCertificateParserFunc(tls.Certificate{}, nil)
	tlscfg, err := tlsInfo.ServerConfig()
	if err != nil {
		t.Fatalf("unexpected serverConfig error: %v", err)
	}

	tlsln, err := NewListenerWithKeepAlive(ln, "https", tlscfg)
	if err != nil {
		t.Fatalf("unexpected NewListenerWithKeepAlive error: %v", err)
	}

	go http.Get("https://" + tlsln.Addr().String())

	conn, err = tlsln.Accept()
	if err != nil {
		t.Fatalf("unexpected Accept error: %v", err)
	}
	if _, ok := conn.(*tls.Conn); !ok {
		t.Errorf("failed to accept *tls.Conn")
	}
	conn.Close()
	tlsln.Close()
}

func Test_NewListenerWithKeepAlive_TLS_EmptyConfig(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("unexpected listen error: %v", err)
	}

	_, err = NewListenerWithKeepAlive(ln, "https", nil)
	if err == nil {
		t.Errorf("err = nil, want not presented error")
	}
}
