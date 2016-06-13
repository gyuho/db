package xlog

import (
	"log"
	"os"
	"sync"
)

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
	// overwrite standard logger
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(stdLogWriter{
		l: NewLogger(""),
	})

	SetFormatter(NewFormatter(os.Stderr, false))
}
