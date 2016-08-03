package netutil

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gyuho/db/pkg/tlsutil"
)

// (etcd pkg.transport.TestNewTransportTLSInfo)
func Test_NewTransport_TLSInfo(t *testing.T) {
	tmp, err := createTempFile([]byte("testdata"))
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp)

	tests := []tlsutil.TLSInfo{
		{},
		{
			CertFile: tmp,
			KeyFile:  tmp,
		},
		{
			CertFile:      tmp,
			KeyFile:       tmp,
			TrustedCAFile: tmp,
		},
		{
			TrustedCAFile: tmp,
		},
	}

	for i, tt := range tests {
		tt.ParseFunc = fakeCertificateParserFunc(tls.Certificate{}, nil)

		trans, err := NewTransport(tt, time.Second)
		if err != nil {
			t.Fatalf("#%d: NewTransport error (%v)", i, err)
		}

		if trans.TLSClientConfig == nil {
			t.Fatalf("#%d: want non-nil TLSClientConfig", i)
		}
	}
}

// (etcd pkg.transport.TestNewTimeoutTransport)
func Test_NewTransportTimeout(t *testing.T) {
	tr, err := NewTransportTimeout(tlsutil.TLSInfo{}, time.Hour, time.Hour, time.Hour)
	if err != nil {
		t.Fatalf("unexpected NewTransportTimeout error: %v", err)
	}

	remoteAddr := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.RemoteAddr))
	}
	srv := httptest.NewServer(http.HandlerFunc(remoteAddr))

	defer srv.Close()
	conn, err := tr.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}
	defer conn.Close()

	tconn, ok := conn.(*connTimeout)
	if !ok {
		t.Fatalf("failed to dial out *connTimeout")
	}
	if tconn.writeTimeout != time.Hour {
		t.Errorf("write timeout = %s, want %s", tconn.writeTimeout, time.Hour)
	}
	if tconn.readTimeout != time.Hour {
		t.Errorf("read timeout = %s, want %s", tconn.readTimeout, time.Hour)
	}

	// ensure not reuse timeout connection
	req, err := http.NewRequest("GET", srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected err %v", err)
	}
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected err %v", err)
	}
	addr0, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("unexpected err %v", err)
	}

	resp, err = tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected err %v", err)
	}
	addr1, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("unexpected err %v", err)
	}

	if bytes.Equal(addr0, addr1) {
		t.Errorf("addr0 = %s addr1= %s, want not equal", string(addr0), string(addr1))
	}
}
