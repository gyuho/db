package tlsutil

import (
	"crypto/tls"
	"fmt"
)

// TLSInfo contains TLS configuration.
//
// (etcd pkg.transport.TLSInfo)
type TLSInfo struct {
	// CertFile is TLS cert file.
	CertFile string

	// KeyFile is TLS key file.
	KeyFile string

	// TrustedCAFile is TLS trusted CA key file.
	TrustedCAFile string

	ClientCertAuth bool

	// ServerName ensures the cert matches the given host
	// in case of discovery / virtual hosting
	ServerName string

	// SelfCert is true, when TLS is self-signed.
	SelfCert bool

	// ParseFunc exists to simplify testing. Typically, parseFunc
	// should be left nil. In that case, tls.X509KeyPair will be used.
	ParseFunc func(certPEMBlock, keyPEMBlock []byte) (tls.Certificate, error)
}

func (ti TLSInfo) String() string {
	return fmt.Sprintf("cert=%q, key=%q, trusted-ca=%q, client-cert-auth=%v", ti.CertFile, ti.KeyFile, ti.TrustedCAFile, ti.ClientCertAuth)
}

// Empty returns true if TLSInfo is empty.
//
// (etcd pkg.transport.TLSInfo.Empty)
func (ti TLSInfo) Empty() bool {
	return ti.CertFile == "" && ti.KeyFile == ""
}

// BaseConfig returns *tls.Config from TLSInfo.
//
// (etcd pkg.transport.TLSInfo.baseConfig)
func (ti TLSInfo) BaseConfig() (*tls.Config, error) {
	if ti.KeyFile == "" || ti.CertFile == "" {
		return nil, fmt.Errorf("CertFile and KeyFile must both be present[cert: %q, key: %q]", ti.CertFile, ti.KeyFile)
	}

	tlsCert, err := NewCert(ti.CertFile, ti.KeyFile, ti.ParseFunc)
	if err != nil {
		return nil, err
	}

	return &tls.Config{Certificates: []tls.Certificate{*tlsCert}, MinVersion: tls.VersionTLS12}, nil
}

// CAFiles returns a list of CA file paths.
//
// (etcd pkg.transport.TLSInfo.cafiles)
func (ti TLSInfo) CAFiles() []string {
	var cs []string
	if ti.TrustedCAFile != "" {
		cs = append(cs, ti.TrustedCAFile)
	}
	return cs
}

// ServerConfig generates a tls.Config object for use by an HTTP server.
//
// (etcd pkg.transport.TLSInfo.ServerConfig)
func (ti TLSInfo) ServerConfig() (*tls.Config, error) {
	cfg, err := ti.BaseConfig()
	if err != nil {
		return nil, err
	}

	cfg.ClientAuth = tls.NoClientCert
	if ti.TrustedCAFile != "" || ti.ClientCertAuth {
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	caFiles := ti.CAFiles()
	if len(caFiles) > 0 {
		cp, err := NewCertPool(caFiles)
		if err != nil {
			return nil, err
		}
		cfg.ClientCAs = cp
	}

	// enable HTTP/2 for Go's HTTP server
	cfg.NextProtos = []string{"h2"}
	return cfg, nil
}

// ClientConfig generates a tls.Config object for use by an HTTP client.
//
// (etcd pkg.transport.TLSInfo.ClientConfig)
func (ti TLSInfo) ClientConfig() (*tls.Config, error) {
	var cfg *tls.Config
	var err error

	if !ti.Empty() {
		cfg, err = ti.BaseConfig()
		if err != nil {
			return nil, err
		}
	} else {
		cfg = &tls.Config{}
	}

	cs := ti.CAFiles()
	if len(cs) > 0 {
		cfg.RootCAs, err = NewCertPool(cs)
		if err != nil {
			return nil, err
		}
	}

	if ti.SelfCert {
		cfg.InsecureSkipVerify = true
	}

	return cfg, nil
}
