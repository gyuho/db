package fileutil

import "os"

// OpenToRead opens a file for reads.
// Make sure to close the file.
func OpenToRead(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_RDONLY, PrivateFileMode)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// OpenToOverwrite creates or opens a file for overwriting.
// Make sure to close the file.
func OpenToOverwrite(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_RDWR|os.O_TRUNC|os.O_CREATE, PrivateFileMode)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// OpenToOverwriteOnly opens a file only for overwriting.
func OpenToOverwriteOnly(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, PrivateFileMode)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// OpenToAppend opens a file for appends. If the file
// does not eixst, it creates one.
// Make sure to close the file.
func OpenToAppend(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_RDWR|os.O_APPEND|os.O_CREATE, PrivateFileMode)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// OpenToAppendOnly opens a file only for appends.
func OpenToAppendOnly(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, PrivateFileMode)
	if err != nil {
		return nil, err
	}
	return f, nil
}
