package tlsutil

import (
	"crypto/tls"
	"errors"
	"os"
	"testing"
)

// (etcd pkg.transport.TestTLSInfoNonexist)
func Test_TLSInfo_Nonexist(t *testing.T) {
	tlsInfo := TLSInfo{CertFile: "@badname", KeyFile: "@badname"}
	_, err := tlsInfo.ServerConfig()
	werr := &os.PathError{
		Op:   "open",
		Path: "@badname",
		Err:  errors.New("no such file or directory"),
	}
	if err.Error() != werr.Error() {
		t.Fatalf("err = %v, want %v", err, werr)
	}
}

// (etcd pkg.transport.TestTLSInfoEmpty)
func Test_TLSInfo_Empty(t *testing.T) {
	tests := []struct {
		info TLSInfo
		want bool
	}{
		{TLSInfo{}, true},
		{TLSInfo{TrustedCAFile: "baz"}, true},
		{TLSInfo{CertFile: "foo"}, false},
		{TLSInfo{KeyFile: "bar"}, false},
		{TLSInfo{CertFile: "foo", KeyFile: "bar"}, false},
		{TLSInfo{CertFile: "foo", TrustedCAFile: "baz"}, false},
		{TLSInfo{KeyFile: "bar", TrustedCAFile: "baz"}, false},
		{TLSInfo{CertFile: "foo", KeyFile: "bar", TrustedCAFile: "baz"}, false},
	}

	for i, tt := range tests {
		got := tt.info.Empty()
		if tt.want != got {
			t.Fatalf("#%d: result of Empty() incorrect: want=%t got=%t", i, tt.want, got)
		}
	}
}

// (etcd pkg.transport.TestTLSInfoMissingFields)
func Test_TLSInfo_MissingFields(t *testing.T) {
	tmp, err := createTempFile([]byte("XXX"))
	if err != nil {
		t.Fatalf("Unable to prepare tmpfile: %v", err)
	}
	defer os.Remove(tmp)

	tests := []TLSInfo{
		{CertFile: tmp},
		{KeyFile: tmp},
		{CertFile: tmp, TrustedCAFile: tmp},
		{KeyFile: tmp, TrustedCAFile: tmp},
	}

	for i, info := range tests {
		if _, err = info.ServerConfig(); err == nil {
			t.Fatalf("#%d: expected non-nil error from ServerConfig()", i)
		}

		if _, err = info.ClientConfig(); err == nil {
			t.Fatalf("#%d: expected non-nil error from ClientConfig()", i)
		}
	}
}

// (etcd pkg.transport.TestTLSInfoParseFuncError)
func Test_TLSInfo_ParseFuncError(t *testing.T) {
	tmp, err := createTempFile([]byte("XXX"))
	if err != nil {
		t.Fatalf("Unable to prepare tmpfile: %v", err)
	}
	defer os.Remove(tmp)

	info := TLSInfo{CertFile: tmp, KeyFile: tmp, TrustedCAFile: tmp}
	info.ParseFunc = fakeCertificateParserFunc(tls.Certificate{}, errors.New("fake"))

	if _, err = info.ServerConfig(); err == nil {
		t.Fatal("expected non-nil error from ServerConfig()")
	}

	if _, err = info.ClientConfig(); err == nil {
		t.Fatal("expected non-nil error from ClientConfig()")
	}
}

// (etcd pkg.transport.TestTLSInfoConfigFuncs)
func Test_TLSInfo_ConfigFuncs(t *testing.T) {
	tmp, err := createTempFile([]byte("XXX"))
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp)

	tests := []struct {
		info       TLSInfo
		clientAuth tls.ClientAuthType
		wantCAs    bool
	}{
		{
			info:       TLSInfo{CertFile: tmp, KeyFile: tmp},
			clientAuth: tls.NoClientCert,
			wantCAs:    false,
		},

		{
			info:       TLSInfo{CertFile: tmp, KeyFile: tmp, TrustedCAFile: tmp},
			clientAuth: tls.RequireAndVerifyClientCert,
			wantCAs:    true,
		},
	}

	for i, tt := range tests {
		tt.info.ParseFunc = fakeCertificateParserFunc(tls.Certificate{}, nil)

		sCfg, err := tt.info.ServerConfig()
		if err != nil {
			t.Fatalf("#%d: expected nil error from ServerConfig(), got non-nil: %v", i, err)
		}

		if tt.wantCAs != (sCfg.ClientCAs != nil) {
			t.Fatalf("#%d: wantCAs=%t but ClientCAs=%v", i, tt.wantCAs, sCfg.ClientCAs)
		}

		cCfg, err := tt.info.ClientConfig()
		if err != nil {
			t.Fatalf("#%d: expected nil error from ClientConfig(), got non-nil: %v", i, err)
		}

		if tt.wantCAs != (cCfg.RootCAs != nil) {
			t.Fatalf("#%d: wantCAs=%t but RootCAs=%v", i, tt.wantCAs, sCfg.RootCAs)
		}
	}
}
