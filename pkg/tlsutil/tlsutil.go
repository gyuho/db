package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
)

// TLSInfo contains TLS configuration.
type TLSInfo struct {
	CertFile       string
	KeyFile        string
	CAFile         string
	TrustedCAFile  string
	ClientCertAuth bool

	SelfCert bool

	// ParseFunc exists to simplify testing. Typically, parseFunc
	// should be left nil. In that case, tls.X509KeyPair will be used.
	ParseFunc func([]byte, []byte) (tls.Certificate, error)
}

func (ti TLSInfo) String() string {
	return fmt.Sprintf("cert=%q, key=%q, ca=%q, trusted-ca=%q, client-cert-auth=%v", ti.CertFile, ti.KeyFile, ti.CAFile, ti.TrustedCAFile, ti.ClientCertAuth)
}

// Empty returns true if TLSInfo is empty.
func (ti TLSInfo) Empty() bool {
	return ti.CertFile == "" && ti.KeyFile == ""
}

// BaseConfig returns *tls.Config from TLSInfo.
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
func (ti TLSInfo) CAFiles() []string {
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
	cfg, err := ti.BaseConfig()
	if err != nil {
		return nil, err
	}

	cfg.ClientAuth = tls.NoClientCert
	if ti.CAFile != "" || ti.ClientCertAuth {
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	cs := ti.CAFiles()
	if len(cs) > 0 {
		cp, err := NewCertPool(cs)
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

// NewCertPool creates x509 certPool with provided CA files.
func NewCertPool(CAFiles []string) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()

	for _, CAFile := range CAFiles {
		pemByte, err := ioutil.ReadFile(CAFile)
		if err != nil {
			return nil, err
		}

		for {
			var block *pem.Block
			block, pemByte = pem.Decode(pemByte)
			if block == nil {
				break
			}
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			certPool.AddCert(cert)
		}
	}

	return certPool, nil
}

// NewCert generates TLS cert by using the given cert,key and parse function.
func NewCert(certFile, keyFile string, parseFunc func([]byte, []byte) (tls.Certificate, error)) (*tls.Certificate, error) {
	cert, err := ioutil.ReadFile(certFile)
	if err != nil {
		return nil, err
	}

	key, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}

	if parseFunc == nil {
		parseFunc = tls.X509KeyPair
	}

	tlsCert, err := parseFunc(cert, key)
	if err != nil {
		return nil, err
	}
	return &tlsCert, nil
}
