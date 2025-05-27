package atomicwriter

import (
	"io"
	"os"

	"github.com/moby/sys/atomicwriter"
)

// New returns a WriteCloser so that writing to it writes to a
// temporary file and closing it atomically changes the temporary file to
// destination path. Writing and closing concurrently is not allowed.
// NOTE: umask is not considered for the file's permissions.
//
// New uses [sequential.CreateTemp] to use sequential file access on Windows,
// avoiding depleting the standby list un-necessarily. On Linux, this equates to
// a regular [os.CreateTemp]. Refer to the [Win32 API documentation] for details
// on sequential file access.
//
// Deprecated: use [atomicwriter.New] instead.
//
// [Win32 API documentation]: https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilea#FILE_FLAG_SEQUENTIAL_SCAN
func New(filename string, perm os.FileMode) (io.WriteCloser, error) {
	return atomicwriter.New(filename, perm)
}

// WriteFile atomically writes data to a file named by filename and with the
// specified permission bits. The given filename is created if it does not exist,
// but the destination directory must exist. It can be used as a drop-in replacement
// for [os.WriteFile], but currently does not allow the destination path to be
// a symlink. WriteFile is implemented using [New] for its implementation.
//
// NOTE: umask is not considered for the file's permissions.
//
// Deprecated: use [atomicwriter.WriteFile] instead.
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return atomicwriter.WriteFile(filename, data, perm)
}

// WriteSet is used to atomically write a set
// of files and ensure they are visible at the same time.
// Must be committed to a new directory.
//
// Deprecated: use [atomicwriter.WriteSet] instead.
type WriteSet = atomicwriter.WriteSet

// NewWriteSet creates a new atomic write set to
// atomically create a set of files. The given directory
// is used as the base directory for storing files before
// commit. If no temporary directory is given the system
// default is used.
//
// Deprecated: use [atomicwriter.NewWriteSet] instead.
func NewWriteSet(tmpDir string) (*atomicwriter.WriteSet, error) {
	return atomicwriter.NewWriteSet(tmpDir)
}
