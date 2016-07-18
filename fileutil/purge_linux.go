package fileutil

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PurgeFile purges files in directory periodically by its suffix.
func PurgeFile(dir, suffix string, max uint, interval time.Duration, stop <-chan struct{}) <-chan error {
	return purgeFile(dir, suffix, max, interval, stop, nil)
}

func purgeFile(dir, suffix string, max uint, interval time.Duration, stop <-chan struct{}, purgec chan<- string) <-chan error {
	errc := make(chan error, 1)
	go func() {
		for {
			fnames, err := ReadDir(dir)
			if err != nil {
				errc <- err
				return
			}

			var ns []string
			for _, fname := range fnames {
				if strings.HasSuffix(fname, suffix) {
					ns = append(ns, fname)
				}
			}
			sort.Strings(ns)

			fnames = ns
			for len(ns) > int(max) {
				f := filepath.Join(dir, ns[0])

				l, err := OpenFileWithLockNonBlocking(f, os.O_WRONLY, PrivateFileMode)
				if err != nil {
					break
				}

				if err = os.Remove(f); err != nil {
					errc <- err
					return
				}

				if err = l.Close(); err != nil {
					logger.Errorf("unlocking error %v when closing %q", err, l.Name())
					errc <- err
					return
				}

				logger.Infof("purged %q", f)

				// pop-front
				ns = ns[1:]
			}

			if purgec != nil {
				for i := 0; i < len(fnames)-len(ns); i++ {
					purgec <- fnames[i]
				}
			}

			select {
			case <-time.After(interval):
			case <-stop:
				logger.Info("purge stopped")
				return
			}
		}
	}()
	return errc
}
