package continuity

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// atomicWriteFile writes data to a file by first writing to a temp
// file and calling rename.
func atomicWriteFile(filename string, r io.Reader, rf RegularFile) error {
	f, err := ioutil.TempFile(filepath.Dir(filename), ".tmp-"+filepath.Base(filename))
	if err != nil {
		return err
	}
	err = os.Chmod(f.Name(), rf.Mode())
	if err != nil {
		f.Close()
		return err
	}
	n, err := io.Copy(f, r)
	if err == nil && n < rf.Size() {
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
