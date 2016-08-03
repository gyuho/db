package netutil

import (
	"crypto/tls"
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
