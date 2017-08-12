package fileutil

import (
	"io"
	"os"
	"syscall"
)

// Preallocate tries to allocate the space for given file.
// If extendFile is true, it calls fallocate without FALLOC_FL_KEEP_SIZE mode,
// which means the file size will be changed depending on the offset.
//
// fallocate preallocates blocks to a file. Disk space is indeed reserved by this call,
// but it doesn't write anything. It allocates blocks and marks them as uninitialized
// requiring no I/O to the data blocks. Which is much faster than creating a file by
// filling it with zeros (pre-allocate pages).
//
// The default operation (mode is 0) of fallocate allocates the disk space
// in the range starting at offset and continuing for sizeInBytes(len) bytes.
// If the size of the file is less than offset + sizeInBytes, then the file is
// increased to this size; otherwise the file size remains unchanged.
// After a successful call, subsequent writes into the range specified by
// offset and sizeInBytes are guaranteed not to fail because of lack of disk
// space.
//
// If FALLOC_FL_KEEP_SIZE flag is specified (mode is 1), the behavior of this
// call is similar, but the file size will not be changed even if the size of
// the file is less than offset + sizeInBytes. Now extra zeroed blocks are
// preallocated beyond the end of this file, which is useful for optimizing
// append workloads.
//
// (http://man7.org/linux/man-pages/man2/fallocate.2.html)
//
func Preallocate(f *os.File, sizeInBytes int64, extendFile bool) error {
	var (
		keepSizeMode uint32
		offset       int64
	)
	if !extendFile {
		keepSizeMode = 1
	}
	err := syscall.Fallocate(int(f.Fd()), keepSizeMode, offset, sizeInBytes)
	if err != nil {
		errno, ok := err.(syscall.Errno)

		if ok {
			switch extendFile {
			case true:
				// fallocate not supported
				// fallocate EINTRs frequently in some environments; fallback
				if errno == syscall.ENOTSUP || errno == syscall.EINTR {
					return preallocExtendTrunc(f, sizeInBytes)
				}

			case false:
				// treat not supported as nil error
				if errno == syscall.ENOTSUP {
					return nil
				}
			}
		}
	}
	return err
}

// preallocExtendTrunc extends the file by adding holes
// without reserving disk space. No actual disk space is reserved.
func preallocExtendTrunc(f *os.File, sizeInBytes int64) error {
	// Seek sets the offset for the next Read or Write on file to offset,
	// interpreted according to whence:
	//
	// move current offset to the beginning (0)
	curOff, err := f.Seek(0, io.SeekCurrent) // 1, io.SeekCurrent: seek relative to the current offset
	if err != nil {
		return err
	}

	// move(set) end of the file with sizeInBytes
	sizeOff, err := f.Seek(sizeInBytes, io.SeekEnd) // 2, io.SeekEnd: seek relative to the end
	if err != nil {
		return err
	}

	// move(set) beginning of the file(io.SeekStart) to curOff(beginning)
	if _, err = f.Seek(curOff, io.SeekStart); err != nil { // 0, io.SeekStart: seek relative to the origin(beginning) of the file
		return err
	}

	if sizeInBytes > sizeOff { // no need to change the file size
		return nil
	}

	// Truncate changes the size of the file.
	return f.Truncate(sizeInBytes)
}
