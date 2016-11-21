package backend

import (
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/boltdb/bolt"
)

var (
	defaultBatchLimit    = 10000
	defaultBatchInterval = 100 * time.Millisecond
)

// InitialMmapSize is the initial size of the mmapped region. Setting this larger than
// the potential max db size can prevent writer from blocking reader.
// This only works for linux.
var InitialMmapSize = int64(10 * 1024 * 1024 * 1024) // 10 GB

// DefaultQuotaBytes is the number of bytes the backend Size may
// consume before exceeding the space quota.
const DefaultQuotaBytes = int64(2 * 1024 * 1024 * 1024) // 2 GB

// MaxQuotaBytes is the maximum number of bytes suggested for a backend
// quota. A larger quota may lead to degraded performance.
const MaxQuotaBytes = int64(8 * 1024 * 1024 * 1024) // 8 GB

type Backend interface {
	BatchTx() BatchTx
	Snapshot() Snapshot
	Hash(ignores map[IgnoreKey]struct{}) (uint32, error)
	Size() int64
	Defrag() error
	ForceCommit()
	Close() error
}

type Snapshot interface {
	Size() int64
	WriteTo(w io.Writer) (n int64, err error)
	Close() error
}

// (etcd mvcc/backend.backend)
type backend struct {
	// size and commits are used with atomic operations so they must be
	// 64-bit aligned, otherwise 32-bit tests will crash

	// size is the number of bytes in the backend
	size int64

	// commits counts number of commits since start
	commits int64

	mu sync.RWMutex
	db *bolt.DB

	batchInterval time.Duration
	batchLimit    int
	batchTx       *batchTx

	stopc chan struct{}
	donec chan struct{}
}

// (etcd mvcc/backend.snapshot)
type snapshot struct {
	*bolt.Tx
}

// (etcd mvcc/backend.snapshot.Close)
func (s *snapshot) Close() error { return s.Tx.Rollback() }

// (etcd mvcc/backend.backend.Close)
func (b *backend) Close() error {
	close(b.stopc)
	<-b.donec
	return b.db.Close()
}

// (etcd mvcc/backend.backend.run)
func (b *backend) run() {
	defer close(b.donec)
	tm := time.NewTimer(b.batchInterval)
	defer tm.Stop()

	for {
		select {
		case <-tm.C:
		case <-b.stopc:
			b.batchTx.CommitAndStop()
			return
		}
		b.batchTx.Commit()
		tm.Reset(b.batchInterval)
	}
}

// syscall.MAP_POPULATE on linux 2.6.23+ does sequential read-ahead
// which can speed up entire-database read with boltdb. We want to
// enable MAP_POPULATE for faster key-value store recovery in storage
// package. If your kernel version is lower than 2.6.23
// (https://github.com/torvalds/linux/releases/tag/v2.6.23), mmap might
// silently ignore this flag. Please update your kernel to prevent this.
var boltOpenOptions = &bolt.Options{
	MmapFlags:       syscall.MAP_POPULATE,
	InitialMmapSize: int(InitialMmapSize),
}

// (etcd mvcc/backend.newBackend)
func newBackend(path string, d time.Duration, limit int) *backend {
	db, err := bolt.Open(path, 0600, boltOpenOptions)
	if err != nil {
		panic(fmt.Errorf("cannot open database at %s (%v)", path, err))
	}

	b := &backend{
		db: db,

		batchInterval: d,
		batchLimit:    limit,

		stopc: make(chan struct{}),
		donec: make(chan struct{}),
	}
	b.batchTx = newBatchTx(b)
	go b.run()
	return b
}

// New returns a new Backend.
//
// (etcd mvcc/backend.New)
func New(path string, d time.Duration, batchLimit int) Backend {
	return newBackend(path, d, batchLimit)
}

// NewDefaultBackend returns a new Backend with default configuration.
//
// (etcd mvcc/backend.NewDefaultBackend)
func NewDefaultBackend(path string) Backend {
	return newBackend(path, defaultBatchInterval, defaultBatchLimit)
}

// NewTmpBackend returns a new temporary backend.
//
// (etcd mvcc/backend.NewTmpBackend)
func NewTmpBackend(batchInterval time.Duration, batchLimit int) (*backend, string) {
	dir, err := ioutil.TempDir(os.TempDir(), "etcd_backend_test")
	if err != nil {
		panic(err)
	}
	tmpPath := filepath.Join(dir, "database")
	return newBackend(tmpPath, batchInterval, batchLimit), tmpPath
}

// NewDefaultTmpBackend returns a new temporary backend with default configuration.
//
// (etcd mvcc/backend.NewDefaultTmpBackend)
func NewDefaultTmpBackend() (*backend, string) {
	return NewTmpBackend(defaultBatchInterval, defaultBatchLimit)
}

// (etcd mvcc/backend.backend.BatchTx)
func (b *backend) BatchTx() BatchTx {
	return b.batchTx
}

// (etcd mvcc/backend.backend.Snapshot)
func (b *backend) Snapshot() Snapshot {
	b.batchTx.Commit()

	b.mu.RLock()
	defer b.mu.RUnlock()
	tx, err := b.db.Begin(false)
	if err != nil {
		panic(fmt.Errorf("cannot begin tx (%s)", err))
	}

	return &snapshot{tx}
}

// (etcd mvcc/backend.backend.Size)
func (b *backend) Size() int64 {
	return atomic.LoadInt64(&b.size)
}

// (etcd mvcc/backend.backend.ForceCommit)
func (b *backend) ForceCommit() {
	b.batchTx.Commit()
}

// (etcd mvcc/backend.backend.Commits)
func (b *backend) Commits() int64 {
	return atomic.LoadInt64(&b.commits)
}

// IgnoreKey defines bucket and key to ignore when calculating hash.
type IgnoreKey struct {
	Bucket string
	Key    string
}

// (etcd mvcc/backend.Hash)
func (b *backend) Hash(ignores map[IgnoreKey]struct{}) (uint32, error) {
	h := crc32.New(crc32.MakeTable(crc32.Castagnoli))

	b.mu.RLock()
	defer b.mu.RUnlock()
	err := b.db.View(func(tx *bolt.Tx) error {
		c := tx.Cursor()
		for next, _ := c.First(); next != nil; next, _ = c.Next() {
			b := tx.Bucket(next)
			if b == nil {
				return fmt.Errorf("cannot get hash of bucket %s", string(next))
			}
			h.Write(next)
			b.ForEach(func(k, v []byte) error {
				bk := IgnoreKey{Bucket: string(next), Key: string(k)}
				if _, ok := ignores[bk]; !ok {
					h.Write(k)
					h.Write(v)
				}
				return nil
			})
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return h.Sum32(), nil
}

func (b *backend) Defrag() error {
	if err := b.defrag(); err != nil {
		return err
	}

	// commit to update metadata like db.size
	b.batchTx.Commit()

	return nil
}

var defragLimit = 10000

// (etcd mvcc/backend.backend.defrag)
func (b *backend) defrag() error {
	b.batchTx.Lock()
	defer b.batchTx.Unlock()

	// lock database after lock tx to avoid deadlock.
	b.mu.Lock()
	defer b.mu.Unlock()

	b.batchTx.commit(true)
	b.batchTx.tx = nil

	tmpdb, err := bolt.Open(b.db.Path()+".tmp", 0600, boltOpenOptions)
	if err != nil {
		return err
	}

	if err = defragdb(b.db, tmpdb, defragLimit); err != nil {
		tmpdb.Close()
		os.RemoveAll(tmpdb.Path())
		return err
	}

	dbp, tdbp := b.db.Path(), tmpdb.Path()

	if err = b.db.Close(); err != nil {
		panic(fmt.Errorf("cannot close database (%s)", err))
	}
	if err = tmpdb.Close(); err != nil {
		panic(fmt.Errorf("cannot close database (%s)", err))
	}

	if err = os.Rename(tdbp, dbp); err != nil {
		panic(fmt.Errorf("cannot rename database (%s)", err))
	}

	b.db, err = bolt.Open(dbp, 0600, boltOpenOptions)
	if err != nil {
		panic(fmt.Errorf("cannot open database at %s (%v)", dbp, err))
	}

	b.batchTx.tx, err = b.db.Begin(true)
	if err != nil {
		panic(fmt.Errorf("cannot begin tx (%s)", err))
	}
	return nil
}

func defragdb(odb, tmpdb *bolt.DB, limit int) error {
	// open a tx on tmpdb for writes
	tmptx, err := tmpdb.Begin(true)
	if err != nil {
		return err
	}

	// open a tx on old db for read
	tx, err := odb.Begin(false)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	c := tx.Cursor()

	count := 0
	for next, _ := c.First(); next != nil; next, _ = c.Next() {
		b := tx.Bucket(next)
		if b == nil {
			return fmt.Errorf("backend: cannot defrag bucket %s", string(next))
		}

		tmpb, berr := tmptx.CreateBucketIfNotExists(next)
		if berr != nil {
			return berr
		}

		b.ForEach(func(k, v []byte) error {
			count++
			if count > limit {
				if err = tmptx.Commit(); err != nil {
					return err
				}

				tmptx, err = tmpdb.Begin(true)
				if err != nil {
					return err
				}
				tmpb = tmptx.Bucket(next)
				count = 0
			}
			return tmpb.Put(k, v)
		})
	}

	return tmptx.Commit()
}
