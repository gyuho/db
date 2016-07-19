package rotate

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gyuho/db/pkg/fileutil"
	"github.com/gyuho/db/pkg/xlog"
)

func TestGetLogName(t *testing.T) {
	name := getLogName()
	if len(name) != 26 {
		t.Fatalf("unexpected %q", name)
	}
}

func TestRotateByFileSize(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "log_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	var (
		rotateFileSize int64 = 100
		rotateDuration       = time.Duration(0)
	)
	ft, err := NewFormatter(Config{
		Dir:            dir,
		RotateFileSize: rotateFileSize,
		RotateDuration: rotateDuration,
	})
	if err != nil {
		t.Fatal(err)
	}

	xlog.SetFormatter(ft)

	logger := xlog.NewLogger("test", xlog.DEBUG)

	logger.Println(strings.Repeat("a", 101))
	logger.Println("Hello World!")
	logger.Println("Hey!")

	fs, err := fileutil.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 2 {
		t.Fatalf("expected 2 files, got %q", fs)
	}

	bts, err := ioutil.ReadFile(filepath.Join(dir, fs[1]))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(bts), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %q", lines)
	}
	if !strings.HasSuffix(lines[1], "Hey!") {
		t.Fatalf("unexpected line %q", lines[1])
	}
}

func TestRotateByDuration(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "log_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	var (
		rotateFileSize int64
		rotateDuration = 50 * time.Millisecond
	)
	ft, err := NewFormatter(Config{
		Dir:            dir,
		RotateFileSize: rotateFileSize,
		RotateDuration: rotateDuration,
	})
	if err != nil {
		t.Fatal(err)
	}

	xlog.SetFormatter(ft)

	logger := xlog.NewLogger("test", xlog.DEBUG)

	time.Sleep(rotateDuration)
	logger.Println("Hello World!")
	logger.Println("Hello World!")
	logger.Println("Hey!")

	fs, err := fileutil.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 2 {
		t.Fatalf("expected 2 files, got %q", fs)
	}

	bts, err := ioutil.ReadFile(filepath.Join(dir, fs[1]))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(bts), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %q", lines)
	}
	if !strings.HasSuffix(lines[1], "Hey!") {
		t.Fatalf("unexpected line %q", lines[1])
	}
}
