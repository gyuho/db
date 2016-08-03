package netutil

import (
	"crypto/tls"
	"os"
	"testing"
	"time"

	"github.com/gyuho/db/pkg/tlsutil"
)

func Test_NewTransport_TLSInfo(t *testing.T) {
	tmp, err := createTempFile([]byte("XXX"))
	if err != nil {
		t.Fatalf("Unable to prepare tmpfile: %v", err)
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
			t.Fatalf("Received unexpected error from NewTransport: %v", err)
		}

		if trans.TLSClientConfig == nil {
			t.Fatalf("#%d: want non-nil TLSClientConfig", i)
		}
	}
}
