package netutil

import (
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"os"
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

// (etcd pkg.transport.testNewListenerTLSInfoAccept)
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
