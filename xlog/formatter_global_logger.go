package xlog

import (
	"log"
	"os"
	"sync"
)

// Formatter defines log-format (printer) interface.
type Formatter interface {
	WriteFlush(pkg string, lvl LogLevel, txt string)
	SetDebug(debug bool)
	Flush()
}

// SetFormatter sets the formatting function for all logs.
func SetFormatter(f Formatter) {
	xlogger.mu.Lock()
	xlogger.formatter = f
	xlogger.mu.Unlock()
}

// SetDebug sets debug for Formatter.
func SetDebug(debug bool) {
	xlogger.mu.Lock()
	xlogger.formatter.SetDebug(debug)
	xlogger.mu.Unlock()
}

type globalLogger struct {
	mu        sync.Mutex
	loggers   map[string]*Logger
	formatter Formatter
}

var xlogger = &globalLogger{
	loggers: make(map[string]*Logger),
}

type stdLogWriter struct {
	l *Logger
}

func (s stdLogWriter) Write(b []byte) (int, error) {
	s.l.log(INFO, string(b))
	return len(b), nil
}

func init() {
	// to overwrite standard logger
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(stdLogWriter{
		l: NewLogger(""),
	})

	SetFormatter(NewDefaultFormatter(os.Stderr, false))
}
