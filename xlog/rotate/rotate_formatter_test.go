package rotate

import (
	"os"
	"testing"
	"time"

	"github.com/gyuho/distdb/xlog"
)

func TestFormatterLogger(t *testing.T) {
	dir := "testdir"
	defer os.RemoveAll(dir)

	ft, err := NewFormatter(Config{
		Dir:            dir,
		RotateFileSize: 64 * 1024 * 1024,
		RotateDuration: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	xlog.SetFormatter(ft)

	logger := xlog.NewLogger("test")
	logger.Println("Hello World!")
	logger.Debugln("DO NOT PRINT THIS")
}
