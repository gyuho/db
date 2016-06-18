package xlog

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestDefaultFormatterLogger(t *testing.T) {
	buf := new(bytes.Buffer)
	SetFormatter(NewDefaultFormatter(buf, false))

	logger := NewLogger("test")
	logger.Println("Hello World!")
	logger.Debugln("DO NOT PRINT THIS")

	txt := buf.String()
	if !strings.Contains(txt, "Hello World!") {
		t.Fatalf("unexpected log %q", txt)
	}
	if strings.Contains(txt, "DO NOT PRINT THIS") {
		t.Fatalf("unexpected log %q", txt)
	}
	fmt.Println(txt)
}

func TestDefaultFormatterLoggerFile(t *testing.T) {
	fpath := "test.log"
	defer os.RemoveAll(fpath)

	f, err := openToAppendOnly(fpath)
	if err != nil {
		t.Fatal(err)
	}
	SetFormatter(NewDefaultFormatter(f, true))

	logger := NewLogger("test")
	logger.Println("Hello World!")
	logger.Debugln("DO NOT PRINT THIS")

	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	b, err := ioutil.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	txt := string(b)

	if !strings.Contains(txt, "Hello World!") {
		t.Fatalf("unexpected log %q", txt)
	}
	if !strings.Contains(txt, "DO NOT PRINT THIS") {
		t.Fatalf("unexpected log %q", txt)
	}
	fmt.Println(txt)
}

func openToAppendOnly(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	return f, nil
}
