package user

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// maxUserFileBytes caps how much data is read from any user-database file.
// User database files are expected to be relatively small. 10 MiB provides
// generous headroom while bounding memory usage.
const maxUserFileBytes = 10 << 20

// openUserFile attempts to open a user-database file with a limitedFile
// capped at maxUserFileBytes. It produces an error if the given path is
// a non-regular file.
func openUserFile(path string) (*limitedFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, &os.PathError{
			Op:   "open",
			Path: path,
			Err:  errors.New("not a regular file"),
		}
	}

	return &limitedFile{
		File: f,
		// Allow one byte past the cap so an overflow surfaces as an
		// error rather than a silent EOF that the parser would treat as
		// a clean end-of-file (and miss any entries past the cap).
		LimitedReader: &io.LimitedReader{R: f, N: maxUserFileBytes + 1},
		name:          path,
	}, nil
}

type limitedFile struct {
	*os.File
	*io.LimitedReader
	name string
}

func (l *limitedFile) Read(p []byte) (int, error) {
	n, err := l.LimitedReader.Read(p)
	if l.LimitedReader.N == 0 {
		return n, &os.PathError{
			Op:   "read",
			Path: l.name,
			Err:  fmt.Errorf("file exceeds %d bytes", maxUserFileBytes),
		}
	}
	return n, err
}
