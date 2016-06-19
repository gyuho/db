package rotate

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gyuho/distdb/xlog"
)

func BenchmarkLogDefault(b *testing.B) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	fpath := filepath.Join(dir, "test.log")

	f, err := openToAppendOnly(fpath)
	if err != nil {
		b.Fatal(err)
	}

	xlog.SetFormatter(xlog.NewDefaultFormatter(f, true))

	logger := xlog.NewLogger("test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Println("TEST")
	}

	if err = f.Close(); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkLogRotate(b *testing.B) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	var (
		rotateFileSize int64 = 3 * 1024 // 3KB
		rotateDuration       = time.Duration(0)
	)
	ft, err := NewFormatter(Config{
		Dir:            dir,
		RotateFileSize: rotateFileSize,
		RotateDuration: rotateDuration,
	})
	if err != nil {
		b.Fatal(err)
	}

	xlog.SetFormatter(ft)

	logger := xlog.NewLogger("test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Println("TEST")
	}
}

func openToAppendOnly(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	return f, nil
}
