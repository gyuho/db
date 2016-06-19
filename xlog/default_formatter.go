package xlog

import (
	"bufio"
	"io"
	"strings"
	"time"
)

type formatter struct {
	w     *bufio.Writer
	debug bool
}

// NewDefaultFormatter returns a new formatter.
func NewDefaultFormatter(w io.Writer, debug bool) Formatter {
	return &formatter{
		w:     bufio.NewWriter(w),
		debug: debug,
	}
}

func (ft *formatter) WriteFlush(pkg string, lvl LogLevel, txt string) {
	if !ft.debug && lvl == DEBUG {
		return
	}

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

func (ft *formatter) SetDebug(debug bool) {
	ft.debug = debug
}

func (ft *formatter) Flush() {
	ft.w.Flush()
}
