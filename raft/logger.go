package raft

import (
	"sync"

	"github.com/gyuho/db/xlog"
)

// Logger defines logging interface for Raft.
type Logger interface {
	Panic(v ...interface{})
	Panicln(v ...interface{})
	Panicf(format string, v ...interface{})

	Fatal(v ...interface{})
	Fatalln(v ...interface{})
	Fatalf(format string, v ...interface{})

	Error(v ...interface{})
	Errorln(v ...interface{})
	Errorf(format string, v ...interface{})

	Warning(v ...interface{})
	Warningln(v ...interface{})
	Warningf(format string, v ...interface{})

	Print(v ...interface{})
	Println(v ...interface{})
	Printf(format string, v ...interface{})

	Info(v ...interface{})
	Infoln(v ...interface{})
	Infof(format string, v ...interface{})

	Debug(v ...interface{})
	Debugln(v ...interface{})
	Debugf(format string, v ...interface{})
}

var (
	raftLogger settableLogger
)

type settableLogger struct {
	mu sync.RWMutex
	l  Logger
}

func (s *settableLogger) SetLogger(l Logger) {
	s.mu.Lock()
	raftLogger.l = l
	s.mu.Unlock()
}

func (s *settableLogger) GetLogger() Logger {
	s.mu.RLock()
	l := raftLogger.l
	s.mu.RUnlock()
	return l
}

func SetLogger(l Logger) {
	raftLogger.SetLogger(l)
}

func GetLogger() Logger {
	return raftLogger.GetLogger()
}

func init() {
	raftLogger.SetLogger(xlog.NewLogger("raft", xlog.DEBUG))
}

func (s *settableLogger) Panic(v ...interface{})                 { s.GetLogger().Panic(v...) }
func (s *settableLogger) Panicf(format string, v ...interface{}) { s.GetLogger().Panicf(format, v...) }
func (s *settableLogger) Panicln(v ...interface{})               { s.GetLogger().Panicln(v...) }
func (s *settableLogger) Fatal(v ...interface{})                 { s.GetLogger().Fatal(v...) }
func (s *settableLogger) Fatalf(format string, v ...interface{}) { s.GetLogger().Fatalf(format, v...) }
func (s *settableLogger) Fatalln(v ...interface{})               { s.GetLogger().Fatalln(v...) }
func (s *settableLogger) Error(v ...interface{})                 { s.GetLogger().Error(v...) }
func (s *settableLogger) Errorf(format string, v ...interface{}) { s.GetLogger().Errorf(format, v...) }
func (s *settableLogger) Errorln(v ...interface{})               { s.GetLogger().Errorln(v...) }
func (s *settableLogger) Warning(v ...interface{})               { s.GetLogger().Warning(v...) }
func (s *settableLogger) Warningf(format string, v ...interface{}) {
	s.GetLogger().Warningf(format, v...)
}
func (s *settableLogger) Warningln(v ...interface{})             { s.GetLogger().Warningln(v...) }
func (s *settableLogger) Print(v ...interface{})                 { s.GetLogger().Print(v...) }
func (s *settableLogger) Printf(format string, v ...interface{}) { s.GetLogger().Printf(format, v...) }
func (s *settableLogger) Println(v ...interface{})               { s.GetLogger().Println(v...) }
func (s *settableLogger) Info(v ...interface{})                  { s.GetLogger().Info(v...) }
func (s *settableLogger) Infof(format string, v ...interface{})  { s.GetLogger().Infof(format, v...) }
func (s *settableLogger) Infoln(v ...interface{})                { s.GetLogger().Infoln(v...) }
func (s *settableLogger) Debug(v ...interface{})                 { s.GetLogger().Debug(v...) }
func (s *settableLogger) Debugf(format string, v ...interface{}) { s.GetLogger().Debugf(format, v...) }
func (s *settableLogger) Debugln(v ...interface{})               { s.GetLogger().Debugln(v...) }
