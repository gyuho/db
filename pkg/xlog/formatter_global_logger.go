package xlog

import (
	"log"
	"os"
	"sync"
)

// Formatter defines log-format (printer) interface.
type Formatter interface {
	// WriteFlush writes the log and flush it to disk.
	// This must be protected by mutex, outside.
	WriteFlush(pkg string, lvl LogLevel, txt string)
	Flush()
}

// SetFormatter sets the formatting function for all logs.
func SetFormatter(f Formatter) {
	xlogger.mu.Lock()
	xlogger.formatter = f
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
		l: NewLogger("", INFO),
	})

	SetFormatter(NewDefaultFormatter(os.Stderr))
}

// GetLogger returns the pkg logger, so that external packages can update the log level.
func GetLogger(name string) (*Logger, bool) {
	xlogger.mu.Lock()
	lg, ok := xlogger.loggers[name]
	xlogger.mu.Unlock()
	return lg, ok
}

// SetGlobalMaxLogLevel sets max log levels of all loggers.
func SetGlobalMaxLogLevel(lvl LogLevel) {
	xlogger.mu.Lock()
	for _, lg := range xlogger.loggers {
		lg.maxLvl = lvl
	}
	xlogger.mu.Unlock()
}
