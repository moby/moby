//go:build windows

package archive

import (
	"os"
	"time"
)

// dirCache on Windows delegates all operations to os.Root methods. There is
// no fd caching because the equivalent openat(2)-style optimisation is not
// available through the current os.Root API on Windows.
type dirCache struct{}

// close is a no-op on Windows.
func (dc *dirCache) close() {}

// openFile opens or creates path within root.
func (dc *dirCache) openFile(root *os.Root, path string, flag int, perm os.FileMode) (*os.File, error) {
	return root.OpenFile(path, flag, perm)
}

// isExistingDir reports whether path within root exists and is a directory.
func (dc *dirCache) isExistingDir(root *os.Root, path string) (bool, error) {
	fi, err := root.Lstat(path)
	if err != nil {
		return false, nil
	}
	return fi.IsDir(), nil
}

// mkdir creates a directory at path within root.
func (dc *dirCache) mkdir(root *os.Root, path string, perm os.FileMode) error {
	return root.Mkdir(path, perm)
}

// lchown sets ownership of path without following symlinks.
func (dc *dirCache) lchown(root *os.Root, path string, uid, gid int) error {
	return root.Lchown(path, uid, gid)
}

// chtimes sets access and modification times of path.
func (dc *dirCache) chtimes(root *os.Root, path string, atime, mtime time.Time) error {
	return root.Chtimes(path, atime, mtime)
}

// lchtimes is a no-op on Windows; symlink timestamps are not supported.
func (dc *dirCache) lchtimes(root *os.Root, path string, atime, mtime time.Time) error {
	return nil
}
