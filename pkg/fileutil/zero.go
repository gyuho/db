package fileutil

import (
	"io"
	"os"
)

/*
https://golang.org/pkg/os/#File.Seek

Seek sets the offset for the next Read or Write on file to offset,
interpreted according to whence:

0 means relative to the origin of the file,
1 means relative to the current offset, and
2 means relative to the end.

It returns the new offset and an error,
if any. The behavior of Seek on a file opened with O_APPEND is not specified.

func (f *File) Seek(offset int64, whence int) (ret int64, err error)

const (
	SEEK_SET int = 0 // seek relative to the origin of the file  == io.SeekStart
	SEEK_CUR int = 1 // seek relative to the current offset      == io.SeekCurrent
	SEEK_END int = 2 // seek relative to the end                 == io.SeekEnd
)


// Seek repositions the offset of the open file from 'whence'
f.Seek(offset, whence)

// file's offset is set to 'offset' bytes
f.Seek(offset, io.SeekStart)

// file's offset is set to its current location + 'offset' bytes
f.Seek(offset, io.SeekCurrent)

// file's offset is set to its file size + 'offset' bytes
// allows the file offset to be set beyond the end of the file
// (but this does not change the size of the file)
f.Seek(offset, io.SeekEnd)

// f.Seek returns offset location in bytes from the beginning of the file
*/

// ZeroToEnd zeros a file from io.SeekCurrent to its io.SeekEnd.
// May temporarily shorten the length of the file.
//
// (etcd pkg.fileutil.ZeroToEnd)
func ZeroToEnd(f *os.File) error {
	// TODO: support FALLOC_FL_ZERO_RANGE
	curOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	endOffset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	if err = f.Truncate(curOffset); err != nil {
		return err
	}

	// make sure blocks remain allocated
	if err = Preallocate(f, endOffset, true); err != nil {
		return err
	}

	_, err = f.Seek(curOffset, io.SeekStart)
	return err
}
