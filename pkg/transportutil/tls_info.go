package transportutil

import (
	"crypto/tls"
	"fmt"

	"github.com/gyuho/db/pkg/tlsutil"
)

// TLSInfo contains TLS configuration.
type TLSInfo struct {
	CertFile       string
	KeyFile        string
	CAFile         string
	TrustedCAFile  string
	ClientCertAuth bool

	selfCert bool

	// parseFunc exists to simplify testing. Typically, parseFunc
	// should be left nil. In that case, tls.X509KeyPair will be used.
	parseFunc func([]byte, []byte) (tls.Certificate, error)
}

func (ti TLSInfo) String() string {
	return fmt.Sprintf("cert=%q, key=%q, ca=%q, trusted-ca=%q, client-cert-auth=%v", ti.CertFile, ti.KeyFile, ti.CAFile, ti.TrustedCAFile, ti.ClientCertAuth)
}

// Empty returns true if TLSInfo is empty.
func (ti TLSInfo) Empty() bool {
	return ti.CertFile == "" && ti.KeyFile == ""
}

func (ti TLSInfo) baseConfig() (*tls.Config, error) {
	if ti.KeyFile == "" || ti.CertFile == "" {
		return nil, fmt.Errorf("CertFile and KeyFile must both be present[cert: %q, key: %q]", ti.CertFile, ti.KeyFile)
	}

	tlsCert, err := tlsutil.NewCert(ti.CertFile, ti.KeyFile, ti.parseFunc)
	if err != nil {
		return nil, err
	}

	return &tls.Config{Certificates: []tls.Certificate{*tlsCert}, MinVersion: tls.VersionTLS12}, nil
}

// cafiles returns a list of CA file paths.
func (ti TLSInfo) caFiles() []string {
	var cs []string
	if ti.CAFile != "" {
		cs = append(cs, ti.CAFile)
	}
	if ti.TrustedCAFile != "" {
		cs = append(cs, ti.TrustedCAFile)
	}
	return cs
}

// ServerConfig generates a tls.Config object for use by an HTTP server.
func (ti TLSInfo) ServerConfig() (*tls.Config, error) {
	cfg, err := ti.baseConfig()
	if err != nil {
		return nil, err
	}

	cfg.ClientAuth = tls.NoClientCert
	if ti.CAFile != "" || ti.ClientCertAuth {
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	cs := ti.caFiles()
	if len(cs) > 0 {
		cp, err := tlsutil.NewCertPool(cs)
		if err != nil {
			return nil, err
		}
		cfg.ClientCAs = cp
	}

	return cfg, nil
}

// ClientConfig generates a tls.Config object for use by an HTTP client.
func (ti TLSInfo) ClientConfig() (*tls.Config, error) {
	var cfg *tls.Config
	var err error

	if !ti.Empty() {
		cfg, err = ti.baseConfig()
		if err != nil {
			return nil, err
		}
	} else {
		cfg = &tls.Config{}
	}

	cs := ti.caFiles()
	if len(cs) > 0 {
		cfg.RootCAs, err = tlsutil.NewCertPool(cs)
		if err != nil {
			return nil, err
		}
	}

	if ti.selfCert {
		cfg.InsecureSkipVerify = true
	}

	return cfg, nil
}
