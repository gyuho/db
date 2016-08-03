package tlsutil

import (
	"io/ioutil"
	"os"
	"testing"
)

// (etcd pkg.transport.TestNewListenerTLSInfoSelfCert)
func Test_TLSInfo_SelfCert(t *testing.T) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "tlsdir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tlsinfo, err := SelfCert(tmpDir, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if tlsinfo.Empty() {
		t.Fatalf("tlsinfo should have certs (%+v)", tlsinfo)
	}
}
