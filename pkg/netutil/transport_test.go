package netutil

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"net"
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

// (etcd pkg.transport.TestReadWriteTimeoutDialer)
func Test_dialerTimeout(t *testing.T) {
	stop := make(chan struct{})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ts := testBlockingServer{ln, 2, stop}
	go ts.Start(t)

	d := dialerTimeout{
		writeTimeout: 10 * time.Millisecond,
		readTimeout:  10 * time.Millisecond,
	}
	conn, err := d.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// fill the socket buffer
	data := make([]byte, 5*1024*1024)
	done := make(chan struct{})
	go func() {
		_, err = conn.Write(data)
		done <- struct{}{}
	}()

	select {
	case <-done:
	// It waits 1s more to avoid delay in low-end system.
	case <-time.After(d.writeTimeout*10 + time.Second):
		t.Fatal("wait timeout")
	}

	if operr, ok := err.(*net.OpError); !ok || operr.Op != "write" || !operr.Timeout() {
		t.Errorf("err = %v, want write i/o timeout error", err)
	}

	conn, err = d.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, 10)
	go func() {
		_, err = conn.Read(buf)
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(d.readTimeout * 10):
		t.Fatal("wait timeout")
	}

	if operr, ok := err.(*net.OpError); !ok || operr.Op != "read" || !operr.Timeout() {
		t.Errorf("err = %v, want write i/o timeout error", err)
	}

	stop <- struct{}{}
}

type testBlockingServer struct {
	ln   net.Listener
	n    int
	stop chan struct{}
}

func (ts *testBlockingServer) Start(t *testing.T) {
	for i := 0; i < ts.n; i++ {
		conn, err := ts.ln.Accept()
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
	}
	<-ts.stop
}
