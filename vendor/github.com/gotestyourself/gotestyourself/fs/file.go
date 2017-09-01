/*Package fs provides tools for creating and working with temporary files and
directories.
*/
package fs

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/stretchr/testify/require"
)

// Path objects return their filesystem path. Both File and Dir implement Path.
type Path interface {
	Path() string
}

// File is a temporary file on the filesystem
type File struct {
	path string
}

// NewFile creates a new file in a temporary directory using prefix as part of
// the filename. The PathOps are applied to the before returning the File.
func NewFile(t require.TestingT, prefix string, ops ...PathOp) *File {
	tempfile, err := ioutil.TempFile("", prefix+"-")
	require.NoError(t, err)
	file := &File{path: tempfile.Name()}
	require.NoError(t, tempfile.Close())

	for _, op := range ops {
		require.NoError(t, op(file))
	}
	return file
}

// Path returns the full path to the file
func (f *File) Path() string {
	return f.path
}

// Remove the file
func (f *File) Remove() {
	// nolint: errcheck
	os.Remove(f.path)
}

// Dir is a temporary directory
type Dir struct {
	path string
}

// NewDir returns a new temporary directory using prefix as part of the directory
// name. The PathOps are applied before returning the Dir.
func NewDir(t require.TestingT, prefix string, ops ...PathOp) *Dir {
	path, err := ioutil.TempDir("", prefix+"-")
	require.NoError(t, err)
	dir := &Dir{path: path}

	for _, op := range ops {
		require.NoError(t, op(dir))
	}
	return dir
}

// Path returns the full path to the directory
func (d *Dir) Path() string {
	return d.path
}

// Remove the directory
func (d *Dir) Remove() {
	// nolint: errcheck
	os.RemoveAll(d.path)
}

// Join returns a new path with this directory as the base of the path
func (d *Dir) Join(parts ...string) string {
	return filepath.Join(append([]string{d.Path()}, parts...)...)
}
