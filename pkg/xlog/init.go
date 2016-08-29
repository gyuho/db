package xlog

import (
	"log"
	"os"
)

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

	wr := stdLogWriter{l: NewLogger("", INFO)}
	log.SetOutput(wr)

	// by default, log-output to stderr
	SetFormatter(NewDefaultFormatter(os.Stderr))
}
