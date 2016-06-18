package rotate

import (
	"bufio"
	"strings"
	"time"

	"github.com/gyuho/distdb/xlog"
)

// Config contains configuration for log rotation.
type Config struct {
	Dir         string
	MaxFileSize int64

	Debug bool
}

type formatter struct {
	w     *bufio.Writer
	debug bool
}

// NewFormatter returns a new formatter.
func NewFormatter(cfg Config) xlog.Formatter {
	return &formatter{}
}

func (rf *formatter) WriteFlush(pkg string, lvl xlog.LogLevel, txt string) {
	if !rf.debug && lvl == xlog.DEBUG {
		return
	}

	rf.w.WriteString(time.Now().String()[:26])
	rf.w.WriteString(" " + lvl.String() + " | ")
	if pkg != "" {
		rf.w.WriteString(pkg + ": ")
	}

	rf.w.WriteString(txt)

	if !strings.HasSuffix(txt, "\n") {
		rf.w.WriteString("\n")
	}

	rf.w.Flush()
}

func (rf *formatter) SetDebug(debug bool) {
	rf.debug = debug
}

func (rf *formatter) Flush() {
	rf.w.Flush()
}
