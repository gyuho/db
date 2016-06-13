package xlog

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestLogger(t *testing.T) {
	buf := new(bytes.Buffer)
	SetFormatter(NewFormatter(buf, false))
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
