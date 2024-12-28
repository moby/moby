package ioutils

import (
	"io"
	"os"

	"github.com/docker/docker/pkg/atomicwriter"
)

// NewAtomicFileWriter returns WriteCloser so that writing to it writes to a
// temporary file and closing it atomically changes the temporary file to
// destination path. Writing and closing concurrently is not allowed.
// NOTE: umask is not considered for the file's permissions.
//
// Deprecated: use [atomicwriter.New] instead.
func NewAtomicFileWriter(filename string, perm os.FileMode) (io.WriteCloser, error) {
	return atomicwriter.New(filename, perm)
}

// AtomicWriteFile atomically writes data to a file named by filename and with the specified permission bits.
// NOTE: umask is not considered for the file's permissions.
//
// Deprecated: use [atomicwriter.WriteFile] instead.
func AtomicWriteFile(filename string, data []byte, perm os.FileMode) error {
	return atomicwriter.WriteFile(filename, data, perm)
}

// AtomicWriteSet is used to atomically write a set
// of files and ensure they are visible at the same time.
// Must be committed to a new directory.
//
// Deprecated: use [atomicwriter.WriteSet] instead.
type AtomicWriteSet = atomicwriter.WriteSet

// NewAtomicWriteSet creates a new atomic write set to
// atomically create a set of files. The given directory
// is used as the base directory for storing files before
// commit. If no temporary directory is given the system
// default is used.
//
// Deprecated: use [atomicwriter.NewWriteSet] instead.
func NewAtomicWriteSet(tmpDir string) (*atomicwriter.WriteSet, error) {
	return atomicwriter.NewWriteSet(tmpDir)
}
