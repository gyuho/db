package raftwal

import (
	"io"
	"os"

	"github.com/gyuho/db/pkg/crcutil"
	"github.com/gyuho/db/pkg/fileutil"
	"github.com/gyuho/db/raftwal/raftwalpb"
)

// Repair repairs the last WAL file by truncating
// for the error io.ErrUnexpectedEOF.
func Repair(dir string) bool {
	f, err := openLastWALFile(dir)
	if err != nil {
		logger.Errorf("failed to open last WAL in %q (%v)", dir, err)
		return false
	}
	defer f.Close()

	dec := newDecoder(f)
	rec := &raftwalpb.Record{}
	for {
		lastValidOffSet := dec.lastValidOffset
		err := dec.decode(rec)
		switch err {
		case nil:
			switch rec.Type {
			case raftwalpb.RECORD_TYPE_CRC:
				cv := dec.crc.Sum32()
				// current CRC of decoder must match
				// CRC of the record.
				// No need to match 0, which means the new one.
				if cv != 0 && rec.Validate(cv) != nil {
					return false
				}

				// update the CRC of the decoder when needed
				dec.crc = crcutil.New(rec.CRC, crcTable)
			}
			continue

		case io.EOF:
			return true

		case io.ErrUnexpectedEOF:
			logger.Infof("repairing %q", f.Name())
			bf, berr := os.Create(f.Name() + ".broken")
			if berr != nil {
				logger.Errorf("failed to create backup %q (%v)", f.Name(), err)
				return false
			}
			defer bf.Close()

			// move(set) beginning of the file(os.SEEK_SET) to offset 0
			if _, err = f.Seek(0, os.SEEK_SET); err != nil {
				logger.Errorf("failed to read backup file %q for WAL (%v)", f.Name(), err)
				return false
			}

			// copy the WAL file to backup file
			if _, err = io.Copy(bf, f); err != nil {
				logger.Errorf("failed to copy WAL file %q to backup file (%v)", f.Name(), err)
				return false
			}

			// Truncate changes the size of the file.
			if err = f.Truncate(int64(lastValidOffSet)); err != nil {
				logger.Errorf("failed to truncate WAL %q (%v)", f.Name(), err)
				return false
			}

			if err = fileutil.Fsync(f.File); err != nil {
				logger.Errorf("failed to fsync %q (%v)", f.Name(), err)
				return false
			}
			return true

		default:
			logger.Errorf("failed to repair %q (%v)", f.Name(), err)
			return false
		}
	}
}
