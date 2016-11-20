package lmdb

// fdatasync flushes written data to a file descriptor.
//
// (boltsync_unix.go fdatasync)
func fdatasync(db *DB) error {
	return db.file.Sync()
}
