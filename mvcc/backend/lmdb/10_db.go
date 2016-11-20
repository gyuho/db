package lmdb

// DB represents the collection of buckets to be persisted on disk.
// All data access is done via transaction which can be obtained from the DB.
// 'ErrDatabaseNotOpen' is returned if DB is accessed before Open is called.
//
// (bolt.DB)
type DB struct {
}
