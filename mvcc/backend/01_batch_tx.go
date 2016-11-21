package backend

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/boltdb/bolt"
)

// BatchTx defines transactions with batch.
//
// (etcd mvcc/backend.BatchTx)
type BatchTx interface {
	Lock()
	Unlock()
	UnsafeCreateBucket(name []byte)
	UnsafePut(bucketName []byte, key []byte, value []byte)
	UnsafeSeqPut(bucketName []byte, key []byte, value []byte)
	UnsafeRange(bucketName []byte, key, endKey []byte, limit int64) (keys [][]byte, vals [][]byte)
	UnsafeDelete(bucketName []byte, key []byte)
	UnsafeForEach(bucketName []byte, visitor func(k, v []byte) error) error
	Commit()
	CommitAndStop()
}

// (etcd mvcc/backend.batchTx)
type batchTx struct {
	mu      sync.Mutex
	tx      *bolt.Tx
	backend *backend
	pending int
}

// (etcd mvcc/backend.batchTx.commit)
func (bt *batchTx) commit(stop bool) {
	var err error
	if bt.tx != nil {
		if bt.pending == 0 && !stop {
			bt.backend.mu.RLock()
			defer bt.backend.mu.RUnlock()

			// batchTx.commit(true) calls *bolt.Tx.Commit, which
			// initializes *bolt.Tx.db and *bolt.Tx.meta as nil,
			// and subsequent *bolt.Tx.Size() call panics.
			//
			// This nil pointer reference panic happens when:
			//   1. batchTx.commit(false) from newBatchTx
			//   2. batchTx.commit(true)  from stopping backend
			//   3. batchTx.commit(false) from inflight mvcc Hash call
			//
			// Check if db is nil to prevent this panic
			if bt.tx.DB() != nil {
				atomic.StoreInt64(&bt.backend.size, bt.tx.Size())
			}
			return
		}

		if err = bt.tx.Commit(); err != nil {
			panic(fmt.Errorf("cannot commit tx %v", err))
		}

		atomic.AddInt64(&bt.backend.commits, 1)
		bt.pending = 0
	}

	if stop {
		// stop before starting a new tx
		// used for stopping backend
		return
	}

	bt.backend.mu.RLock()
	defer bt.backend.mu.RUnlock()

	bt.tx, err = bt.backend.db.Begin(true)
	if err != nil {
		panic(fmt.Errorf("cannot begin tx %v", err))
	}
	atomic.StoreInt64(&bt.backend.size, bt.tx.Size())
}

func (bt *batchTx) Lock() {
	bt.mu.Lock()
}

// (etcd mvcc/backend.batchTx.Unlock)
func (bt *batchTx) Unlock() {
	if bt.pending >= bt.backend.batchLimit { // if pending has reached batchLimit, commit
		bt.commit(false)
		bt.pending = 0
	}
	bt.mu.Unlock()
}

// (etcd mvcc/backend.batchTx.Commit)
func (bt *batchTx) Commit() {
	bt.Lock()
	bt.commit(false)
	bt.Unlock()
}

// (etcd mvcc/backend.batchTx.CommitAndStop)
func (bt *batchTx) CommitAndStop() {
	bt.Lock()
	bt.commit(true)
	bt.Unlock()
}

// (etcd mvcc/backend.newBatchTx)
func newBatchTx(be *backend) *batchTx {
	tx := &batchTx{backend: be}
	tx.Commit()
	return tx
}

// (etcd mvcc/backend.batchTx.UnsafeCreateBucket)
func (bt *batchTx) UnsafeCreateBucket(name []byte) {
	if _, err := bt.tx.CreateBucket(name); err != nil && err != bolt.ErrBucketExists {
		panic(fmt.Errorf("cannot create bucket %q (%v)", name, err))
	}
	bt.pending++
}

// (etcd mvcc/backend.batchTx.unsafePut)
func (bt *batchTx) unsafePut(bucketName []byte, key []byte, value []byte, seq bool) {
	bucket := bt.tx.Bucket(bucketName)
	if bucket == nil {
		panic(fmt.Errorf("bucket %s does not exist", bucketName))
	}

	if seq {
		// it is useful to increase fill percent when the workloads are mostly append-only.
		// this can delay the page split and reduce space usage.
		bucket.FillPercent = 0.9
	}

	if err := bucket.Put(key, value); err != nil {
		panic(fmt.Errorf("cannot put key into bucket (%v)", err))
	}

	bt.pending++
}

// (etcd mvcc/backend.batchTx.UnsafePut)
func (bt *batchTx) UnsafePut(bucketName []byte, key []byte, value []byte) {
	bt.unsafePut(bucketName, key, value, false)
}

// (etcd mvcc/backend.batchTx.UnsafeSeqPut)
func (bt *batchTx) UnsafeSeqPut(bucketName []byte, key []byte, value []byte) {
	bt.unsafePut(bucketName, key, value, true)
}

// (etcd mvcc/backend.batchTx.UnsafeRange)
func (bt *batchTx) UnsafeRange(bucketName []byte, key, endKey []byte, limit int64) (keys [][]byte, vals [][]byte) {
	bucket := bt.tx.Bucket(bucketName)
	if bucket == nil {
		panic(fmt.Errorf("bucket %s does not exist", bucketName))
	}

	if len(endKey) == 0 {
		val := bucket.Get(key)
		if val == nil {
			return
		}
		return append(keys, key), append(vals, val)
	}

	c := bucket.Cursor()

	// bytes.Compare returns -1 if ck < endKey
	for ck, cv := c.Seek(key); ck != nil && bytes.Compare(ck, endKey) < 0; ck, cv = c.Next() {
		keys, vals = append(keys, ck), append(vals, cv)
		if limit > 0 && limit == int64(len(keys)) {
			break
		}
	}
	return
}

// (etcd mvcc/backend.batchTx.UnsafeDelete)
func (bt *batchTx) UnsafeDelete(bucketName []byte, key []byte) {
	bucket := bt.tx.Bucket(bucketName)
	if bucket == nil {
		panic(fmt.Errorf("bucket %s does not exist", bucketName))
	}

	if err := bucket.Delete(key); err != nil {
		panic(fmt.Errorf("cannot delete key from bucket (%v)", err))
	}

	bt.pending++
}

// (etcd mvcc/backend.batchTx.UnsafeForEach)
func (bt *batchTx) UnsafeForEach(bucketName []byte, visitor func(k, v []byte) error) error {
	b := bt.tx.Bucket(bucketName)
	if b == nil {
		// bucket does not exist
		return nil
	}

	return b.ForEach(visitor)
}
