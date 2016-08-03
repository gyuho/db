package netutil

import (
	"crypto/tls"
	"io/ioutil"
)

func createTempFile(b []byte) (string, error) {
	f, err := ioutil.TempFile("", "tls-tests")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err = f.Write(b); err != nil {
		return "", err
	}

	return f.Name(), nil
}

func fakeCertificateParserFunc(cert tls.Certificate, err error) func(certPEMBlock, keyPEMBlock []byte) (tls.Certificate, error) {
	return func(certPEMBlock, keyPEMBlock []byte) (tls.Certificate, error) {
		return cert, err
	}
}
