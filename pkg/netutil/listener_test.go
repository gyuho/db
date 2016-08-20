package netutil

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"

	"github.com/gyuho/db/pkg/tlsutil"
)

func Test_NewListener(t *testing.T) {
	// stopc := make(chan struct{})
	// ln, err := NewListenerStoppable("http", "127.0.0.1:0", nil, stopc)

	ln, err := NewListener("http", "127.0.0.1:0", nil)
	if err != nil {
		t.Fatal(err)
	}

	connw, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = connw.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err = connw.Close(); err != nil {
		t.Fatal(err)
	}

	connl, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	bts := make([]byte, 5)
	if _, err = connl.Read(bts); err != nil {
		t.Fatal(err)
	}
	if err = connl.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bts, []byte("hello")) {
		t.Fatalf("expected %q, got %q", "hello", string(bts))
	}
}

func Test_NewListenerStoppable(t *testing.T) {
	stopc := make(chan struct{})
	ln, err := NewListenerStoppable("http", "127.0.0.1:0", nil, stopc)
	if err != nil {
		t.Fatal(err)
	}

	connw, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = connw.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err = connw.Close(); err != nil {
		t.Fatal(err)
	}

	connl, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	bts := make([]byte, 5)
	if _, err = connl.Read(bts); err != nil {
		t.Fatal(err)
	}
	if err = connl.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bts, []byte("hello")) {
		t.Fatalf("expected %q, got %q", "hello", string(bts))
	}
}

func Test_NewListenerStoppable_stop(t *testing.T) {
	stopc := make(chan struct{})
	ln, err := NewListenerStoppable("http", "127.0.0.1:0", nil, stopc)
	if err != nil {
		t.Fatal(err)
	}

	connw, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = connw.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}

	close(stopc)

	if _, err = ln.Accept(); err != ErrListenerStopped {
		t.Fatalf("expected %v, got %v", ErrListenerStopped, err)
	}
}

// (etcd pkg.transport.TestNewListenerUnixSocket)
func Test_NewListenerUnix(t *testing.T) {
	l, err := NewListener("unix", "testsocket-address", nil)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
}

// (etcd pkg.transport.TestNewListenerTLSEmptyInfo)
func Test_NewListener_Empty_TLSInfo(t *testing.T) {
	if _, err := NewListener("https", "127.0.0.1:0", nil); err == nil {
		t.Fatal("err = nil, want not presented error")
	}
}

// (etcd pkg.transport.testNewListenerTLSInfoAccept)
func testNewListenerTLSInfoAccept(t *testing.T, tlsInfo tlsutil.TLSInfo) {
	tlsServerCfg, err := tlsInfo.ServerConfig()
	if err != nil {
		t.Fatal(err)
	}

	ln, err := NewListener("https", "127.0.0.1:0", tlsServerCfg)
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

// (etcd pkg.transport.TestNewListenerTLSInfo)
func Test_NewListener_TLSInfo(t *testing.T) {
	tmp, err := createTempFile([]byte("XXX"))
	if err != nil {
		t.Fatalf("unable to create tmpfile: %v", err)
	}
	defer os.Remove(tmp)

	tlsInfo := tlsutil.TLSInfo{CertFile: tmp, KeyFile: tmp}
	tlsInfo.ParseFunc = fakeCertificateParserFunc(tls.Certificate{}, nil)

	testNewListenerTLSInfoAccept(t, tlsInfo)
}

// (etcd pkg.transport.TestNewListenerTLSInfoSelfCert)
func Test_NewListener_TLSInfo_SelfCert(t *testing.T) {
	tmp, err := ioutil.TempDir(os.TempDir(), "tlsdir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	tlsinfo, err := tlsutil.SelfCert(tmp, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if tlsinfo.Empty() {
		t.Fatalf("tlsinfo should have certs (%+v)", tlsinfo)
	}

	testNewListenerTLSInfoAccept(t, tlsinfo)
}
