package xlog

import (
	"fmt"
	"os"
)

// LogLevel is the set of all log levels.
type LogLevel int8

const (
	// CRITICAL is the lowest log level. Will exit the program.
	CRITICAL LogLevel = iota - 1

	// ERROR is for errors, but does not fatal. Only indicates potential troubles.
	ERROR

	// WARN warns about potential errors or problems.
	WARN

	// INFO just indicates information.
	INFO

	// DEBUG is debug-level logging, hidden by default.
	DEBUG
)

// String returns a single-character representation of LogLevel.
func (l LogLevel) String() string {
	switch l {
	case CRITICAL:
		return "C"
	case ERROR:
		return "E"
	case WARN:
		return "W"
	case INFO:
		return "I"
	case DEBUG:
		return "D"
	default:
		panic("unknown LogLevel")
	}
}

// Logger contains log prefix(pkg) and LogLevel.
type Logger struct {
	pkg string
}

// NewLogger returns a Logger with pkg prefix.
func NewLogger(pkg string) *Logger {
	xlogger.mu.Lock()
	defer xlogger.mu.Unlock()

	lg, ok := xlogger.loggers[pkg]
	if !ok {
		lg = &Logger{pkg: pkg}
		xlogger.loggers[pkg] = lg
	}

	return lg
}

func (l *Logger) log(lvl LogLevel, txt string) {
	xlogger.mu.Lock()
	defer xlogger.mu.Unlock()

	if lvl < CRITICAL || lvl > DEBUG {
		return
	}

	xlogger.formatter.WriteFlush(l.pkg, lvl, txt)
}

func (l *Logger) Panic(args ...interface{}) {
	txt := fmt.Sprint(args...)
	l.log(CRITICAL, txt)
	panic(txt)
}

func (l *Logger) Panicln(args ...interface{}) {
	txt := fmt.Sprintln(args...)
	l.log(CRITICAL, txt)
	panic(txt)
}

func (l *Logger) Panicf(format string, args ...interface{}) {
	txt := fmt.Sprintf(format, args...)
	l.log(CRITICAL, txt)
	panic(txt)
}

func (l *Logger) Fatal(args ...interface{}) {
	txt := fmt.Sprint(args...)
	l.log(CRITICAL, txt)
	os.Exit(1)
}

func (l *Logger) Fatalln(args ...interface{}) {
	txt := fmt.Sprintln(args...)
	l.log(CRITICAL, txt)
	os.Exit(1)
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	txt := fmt.Sprintf(format, args...)
	l.log(CRITICAL, txt)
	os.Exit(1)
}

func (l *Logger) Error(args ...interface{}) {
	txt := fmt.Sprint(args...)
	l.log(ERROR, txt)
}

func (l *Logger) Errorln(args ...interface{}) {
	txt := fmt.Sprintln(args...)
	l.log(ERROR, txt)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	txt := fmt.Sprintf(format, args...)
	l.log(ERROR, txt)
}

func (l *Logger) Warning(args ...interface{}) {
	txt := fmt.Sprint(args...)
	l.log(WARN, txt)
}

func (l *Logger) Warningln(args ...interface{}) {
	txt := fmt.Sprintln(args...)
	l.log(WARN, txt)
}

func (l *Logger) Warningf(format string, args ...interface{}) {
	txt := fmt.Sprintf(format, args...)
	l.log(WARN, txt)
}

func (l *Logger) Print(args ...interface{}) {
	txt := fmt.Sprint(args...)
	l.log(INFO, txt)
}

func (l *Logger) Println(args ...interface{}) {
	txt := fmt.Sprintln(args...)
	l.log(INFO, txt)
}

func (l *Logger) Printf(format string, args ...interface{}) {
	txt := fmt.Sprintf(format, args...)
	l.log(INFO, txt)
}

func (l *Logger) Info(args ...interface{}) {
	txt := fmt.Sprint(args...)
	l.log(INFO, txt)
}

func (l *Logger) Infoln(args ...interface{}) {
	txt := fmt.Sprintln(args...)
	l.log(INFO, txt)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	txt := fmt.Sprintf(format, args...)
	l.log(INFO, txt)
}

func (l *Logger) Debug(args ...interface{}) {
	txt := fmt.Sprint(args...)
	l.log(DEBUG, txt)
}

func (l *Logger) Debugln(args ...interface{}) {
	txt := fmt.Sprintln(args...)
	l.log(DEBUG, txt)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	txt := fmt.Sprintf(format, args...)
	l.log(DEBUG, txt)
}
