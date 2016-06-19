package rotate

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gyuho/distdb/fileutil"
	"github.com/gyuho/distdb/xlog"
)

var numRegex = regexp.MustCompile("[0-9]+")

// getLogName generates a log file name based on current time
// (e.g. 20160619-142321-397035.log).
func getLogName() string {
	txt := time.Now().String()[:26]
	txt = strings.Join(numRegex.FindAllString(txt, -1), "")
	return txt[:8] + "-" + txt[8:14] + "-" + txt[14:] + ".log"
}

// Config contains configuration for log rotation.
type Config struct {
	// Dir is the directory to put log files.
	Dir   string
	Debug bool

	RotateFileSize int64
	RotateDuration time.Duration
}

type formatter struct {
	dir   string
	debug bool

	rotateFileSize int64

	rotateDuration time.Duration
	started        time.Time

	w    *bufio.Writer
	file *fileutil.LockedFile
}

// NewFormatter returns a new formatter.
func NewFormatter(cfg Config) (xlog.Formatter, error) {
	if err := fileutil.MkdirAll(cfg.Dir); err != nil {
		return nil, err
	}

	// create temporary directory, and rename later to make it appear atomic
	tmpDir := filepath.Clean(cfg.Dir) + ".tmp"
	if fileutil.ExistFile(tmpDir) {
		if err := os.RemoveAll(tmpDir); err != nil {
			return nil, err
		}
	}
	if err := fileutil.MkdirAll(tmpDir); err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	var (
		logName    = getLogName()
		logPath    = filepath.Join(cfg.Dir, logName)
		logPathTmp = filepath.Join(tmpDir, logName)
	)
	f, err := fileutil.LockFile(logPathTmp, os.O_WRONLY|os.O_CREATE, fileutil.PrivateFileMode)
	if err != nil {
		return nil, err
	}
	if err = os.Rename(logPathTmp, logPath); err != nil {
		return nil, err
	}

	ft := &formatter{
		dir:            cfg.Dir,
		debug:          cfg.Debug,
		rotateFileSize: cfg.RotateFileSize,
		rotateDuration: cfg.RotateDuration,
		started:        time.Now(),
		w:              bufio.NewWriter(f),
		file:           f,
	}
	return ft, nil
}

// WriteFlush writes log and flushes it. This is protected by mutex in xlog.
func (ft *formatter) WriteFlush(pkg string, lvl xlog.LogLevel, txt string) {
	if !ft.debug && lvl == xlog.DEBUG {
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

	// fsync to the disk
	ft.w.Flush()

	// seek the current location, and get the offset
	curOffset, err := ft.file.Seek(0, os.SEEK_CUR)
	if err != nil {
		panic(err)
	}

	var (
		needRotateBySize     = ft.rotateFileSize > 0 && curOffset > ft.rotateFileSize
		needRotateByDuration = ft.rotateDuration > time.Duration(0) && time.Since(ft.started) > ft.rotateDuration
	)
	if needRotateBySize || needRotateByDuration {
		ft.unsafeRotate()
	}
}

func (ft *formatter) unsafeRotate() {
	// unlock the locked file
	if err := ft.file.Close(); err != nil {
		panic(err)
	}

	var (
		logPath    = filepath.Join(ft.dir, getLogName())
		logPathTmp = logPath + ".tmp"
	)
	fileTmp, err := fileutil.LockFile(logPathTmp, os.O_WRONLY|os.O_CREATE, fileutil.PrivateFileMode)
	if err != nil {
		panic(err)
	}

	// rename the file to WAL name atomically
	if err = os.Rename(logPathTmp, logPath); err != nil {
		panic(err)
	}

	// release the lock, flush buffer
	if err = fileTmp.Close(); err != nil {
		panic(err)
	}

	// create a new locked file for appends
	fileTmp, err = fileutil.LockFile(logPath, os.O_WRONLY, fileutil.PrivateFileMode)
	if err != nil {
		panic(err)
	}

	ft.w = bufio.NewWriter(fileTmp)
	ft.file = fileTmp

	ft.started = time.Now()
}

func (ft *formatter) SetDebug(debug bool) {
	ft.debug = debug
}

func (ft *formatter) Flush() {
	ft.w.Flush()
}
