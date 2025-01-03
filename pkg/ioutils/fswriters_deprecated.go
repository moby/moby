package ioutils

import (
	"io"
	"os"

	"github.com/docker/docker/pkg/fswriter"
)

// NewAtomicFileWriter returns WriteCloser so that writing to it writes to a
// temporary file and closing it atomically changes the temporary file to
// destination path. Writing and closing concurrently is not allowed.
// NOTE: umask is not considered for the file's permissions.
//
// Deprecated: use [fswriter.NewAtomicFileWriter] instead.
func NewAtomicFileWriter(filename string, perm os.FileMode) (io.WriteCloser, error) {
	return fswriter.NewAtomicFileWriter(filename, perm)
}

// AtomicWriteFile atomically writes data to a file named by filename and with the specified permission bits.
// NOTE: umask is not considered for the file's permissions.
//
// Deprecated: use [fswriter.AtomicWriteFile] instead.
func AtomicWriteFile(filename string, data []byte, perm os.FileMode) error {
	return fswriter.AtomicWriteFile(filename, data, perm)
}

// AtomicWriteSet is used to atomically write a set
// of files and ensure they are visible at the same time.
// Must be committed to a new directory.
//
// Deprecated: use [fswriter.AtomicWriteSet] instead.
type AtomicWriteSet = fswriter.AtomicWriteSet

// NewAtomicWriteSet creates a new atomic write set to
// atomically create a set of files. The given directory
// is used as the base directory for storing files before
// commit. If no temporary directory is given the system
// default is used.
//
// Deprecated: use [fswriter.NewAtomicWriteSet] instead.
func NewAtomicWriteSet(tmpDir string) (*fswriter.AtomicWriteSet, error) {
	return fswriter.NewAtomicWriteSet(tmpDir)
}
