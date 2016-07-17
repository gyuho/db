package xlog

import (
	"bufio"
	"io"
	"strings"
	"time"
)

type formatter struct {
	w *bufio.Writer
}

// NewDefaultFormatter returns a new formatter.
func NewDefaultFormatter(w io.Writer) Formatter {
	return &formatter{
		w: bufio.NewWriter(w),
	}
}

func (ft *formatter) WriteFlush(pkg string, lvl LogLevel, txt string) {
	ft.w.WriteString(time.Now().String()[:26])
	ft.w.WriteString(" " + lvl.String() + " | ")
	if pkg != "" {
		ft.w.WriteString(pkg + ": ")
	}

	ft.w.WriteString(txt)

	if !strings.HasSuffix(txt, "\n") {
		ft.w.WriteString("\n")
	}

	ft.w.Flush()
}

func (ft *formatter) Flush() {
	ft.w.Flush()
}
