package xlog

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"time"
)

// Formatter defines log-format (printer) interface.
type Formatter interface {
	// WriteFlush writes the log and flush it to disk.
	// This must be protected by mutex, outside.
	WriteFlush(pkg string, lvl LogLevel, txt string)
	Flush()
}

//////////////////////////////////////////////////////

type defaultFormatter struct {
	w *bufio.Writer
}

// NewDefaultFormatter returns a new formatter.
func NewDefaultFormatter(w io.Writer) Formatter {
	return &defaultFormatter{
		w: bufio.NewWriter(w),
	}
}

func (ft *defaultFormatter) WriteFlush(pkg string, lvl LogLevel, txt string) {
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

func (ft *defaultFormatter) Flush() {
	ft.w.Flush()
}

//////////////////////////////////////////////////////

type jsonFormatter struct {
	w *bufio.Writer
}

// NewJSONFormatter returns a new formatter.
func NewJSONFormatter(w io.Writer) Formatter {
	return &jsonFormatter{
		w: bufio.NewWriter(w),
	}
}

type jsonFormat struct {
	Pkg   string `json:"pkg"`
	Level string `json:"level"`
	Time  string `json:"time"`
	Log   string `json:"log"`
}

func (ft *jsonFormatter) WriteFlush(pkg string, lvl LogLevel, txt string) {
	l := jsonFormat{
		Pkg:   pkg,
		Level: lvl.String(),
		Time:  time.Now().String()[:26],
		Log:   txt,
	}
	json.NewEncoder(ft.w).Encode(l)
	ft.w.Flush()
}

func (ft *jsonFormatter) Flush() {
	ft.w.Flush()
}

//////////////////////////////////////////////////////

// SetFormatter sets the formatting function for all logs.
func SetFormatter(f Formatter) {
	xlogger.mu.Lock()
	xlogger.formatter = f
	xlogger.mu.Unlock()
}
