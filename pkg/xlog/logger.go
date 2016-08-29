package xlog

import (
	"fmt"
	"os"
	"sync"
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

	// DEBUG is debug-level logging.
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
	pkg    string
	maxLvl LogLevel
}

//////////////////////////////////////////////////////

func (l *Logger) log(lvl LogLevel, txt string) {
	if lvl < CRITICAL || lvl > DEBUG {
		return
	}

	xlogger.mu.Lock()
	if l.maxLvl < lvl {
		xlogger.mu.Unlock()
		return
	}
	xlogger.formatter.WriteFlush(l.pkg, lvl, txt)
	xlogger.mu.Unlock()
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

//////////////////////////////////////////////////////

type globalLogger struct {
	mu        sync.Mutex
	loggers   map[string]*Logger
	formatter Formatter
}

var xlogger = &globalLogger{
	loggers: make(map[string]*Logger),
}

// SetMaxLogLevel updates logger's LogLevel.
func (l *Logger) SetMaxLogLevel(lvl LogLevel) {
	xlogger.mu.Lock()
	l.maxLvl = lvl
	xlogger.mu.Unlock()
}

// SetGlobalMaxLogLevel sets max log levels of all loggers.
func SetGlobalMaxLogLevel(lvl LogLevel) {
	xlogger.mu.Lock()
	for _, lg := range xlogger.loggers {
		lg.maxLvl = lvl
	}
	xlogger.mu.Unlock()
}

// GetLogger returns the pkg logger, so that external packages can update the log level.
func GetLogger(name string) (*Logger, bool) {
	xlogger.mu.Lock()
	lg, ok := xlogger.loggers[name]
	xlogger.mu.Unlock()
	return lg, ok
}

// NewLogger returns a Logger with pkg prefix.
func NewLogger(pkg string, maxLvl LogLevel) *Logger {
	lg := &Logger{pkg: pkg, maxLvl: maxLvl}

	xlogger.mu.Lock() // overwrite
	xlogger.loggers[pkg] = lg
	xlogger.mu.Unlock()

	return lg
}
