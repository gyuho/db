package rotate

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gyuho/distdb/fileutil"
	"github.com/gyuho/distdb/xlog"
)

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

	lockedFile *fileutil.LockedFile
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
		tmpLogPath = filepath.Join(tmpDir, logName)
		logPath    = filepath.Join(cfg.Dir, logName)
	)
	f, err := fileutil.LockFile(tmpLogPath, os.O_WRONLY|os.O_CREATE, fileutil.PrivateFileMode)
	if err != nil {
		return nil, err
	}

	// set offset to the end of file with 0 for pre-allocation
	if _, err := f.Seek(0, os.SEEK_END); err != nil {
		return nil, err
	}
	if err := fileutil.Preallocate(f.File, cfg.RotateFileSize, true); err != nil {
		return nil, err
	}

	if err := os.Rename(tmpLogPath, logPath); err != nil {
		return nil, err
	}

	ft := &formatter{
		dir:            cfg.Dir,
		debug:          cfg.Debug,
		rotateFileSize: cfg.RotateFileSize,
		rotateDuration: cfg.RotateDuration,
		lockedFile:     f,
	}
	// w.filePipeline = newFilePipeline(dir, segmentSizeBytes)

	return ft, nil
}

func (ft *formatter) WriteFlush(pkg string, lvl xlog.LogLevel, txt string) {
	if !ft.debug && lvl == xlog.DEBUG {
		return
	}

	ft.lockedFile.WriteString(time.Now().String()[:26])
	ft.lockedFile.WriteString(" " + lvl.String() + " | ")
	if pkg != "" {
		ft.lockedFile.WriteString(pkg + ": ")
	}

	ft.lockedFile.WriteString(txt)

	if !strings.HasSuffix(txt, "\n") {
		ft.lockedFile.WriteString("\n")
	}

	fileutil.Fdatasync(ft.lockedFile.File)
}

func (ft *formatter) SetDebug(debug bool) {
	ft.debug = debug
}

func (ft *formatter) Flush() {
	fileutil.Fdatasync(ft.lockedFile.File)
}
