package ioutils

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// todo: split docker/pkg/ioutils into a separate repo

// AtomicWriteFile atomically writes data to a file specified by filename.
func AtomicWriteFile(filename string, data []byte, perm os.FileMode) error {
	f, err := ioutil.TempFile(filepath.Dir(filename), ".tmp-"+filepath.Base(filename))
	if err != nil {
		return err
	}
	err = os.Chmod(f.Name(), perm)
	if err != nil {
		f.Close()
		return err
	}
	n, err := f.Write(data)
	if err == nil && n < len(data) {
		f.Close()
		return io.ErrShortWrite
	}
	if err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filename)
}
