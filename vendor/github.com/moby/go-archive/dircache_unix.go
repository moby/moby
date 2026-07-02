//go:build !windows

package archive

import (
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// dirCache caches an open *os.File for the most recently accessed parent
// directory, eliminating repeated doInRoot path walks for consecutive tar
// entries in the same directory. doInRoot opens every path component on
// each call; with a cached parent fd, operations on the final component
// cost a single *at(2) syscall regardless of path depth.
//
// The cache is keyed on both the *os.Root and the root-relative directory
// path, so it invalidates automatically when the root changes (e.g. when a
// separate root is used for AUFS temporary files).
type dirCache struct {
	root *os.Root // root under which the cached dir was opened
	path string   // root-relative path of the cached directory
	file *os.File // open directory fd; nil when nothing is cached
}

// close releases the cached directory fd.
func (dc *dirCache) close() {
	if dc.file != nil {
		dc.file.Close()
		dc.file = nil
		dc.path = ""
		dc.root = nil
	}
}

// splitPath returns the parent directory and base name for path.
func splitPath(path string) (dir, base string) {
	return filepath.Dir(path), filepath.Base(path)
}

// openDir returns an open *os.File for dir within root, reusing the cached
// fd when possible and re-opening via root.OpenFile otherwise.
func (dc *dirCache) openDir(root *os.Root, dir string) (*os.File, error) {
	if dc.file != nil && dc.root == root && dc.path == dir {
		return dc.file, nil
	}
	if dc.file != nil {
		dc.file.Close()
		dc.file = nil
	}
	// Open through root so that path resolution is bounded within the
	// root's security boundary before we cache the raw fd.
	f, err := root.OpenFile(dir, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	dc.file = f
	dc.path = dir
	dc.root = root
	return f, nil
}

// openFile creates or truncates the file at path within root, using
// openat(2) on the cached parent directory fd.
func (dc *dirCache) openFile(root *os.Root, path string, flag int, perm os.FileMode) (*os.File, error) {
	dir, base := splitPath(path)
	d, err := dc.openDir(root, dir)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Openat(int(d.Fd()), base, flag|syscall.O_CLOEXEC, uint32(perm))
	if err != nil {
		return nil, &os.PathError{Op: "openat", Path: path, Err: err}
	}
	return os.NewFile(uintptr(fd), path), nil
}

// isExistingDir reports whether path within root exists and is a directory.
// A false return with a nil error means the path does not exist or is not a
// directory; the caller should attempt to create it.
func (dc *dirCache) isExistingDir(root *os.Root, path string) (bool, error) {
	dir, base := splitPath(path)
	d, err := dc.openDir(root, dir)
	if err != nil {
		return false, err
	}
	var stat unix.Stat_t
	if err := unix.Fstatat(int(d.Fd()), base, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		// Any stat error is treated as "not an existing directory"; the
		// subsequent mkdir call will produce a real error if needed.
		return false, nil
	}
	return stat.Mode&syscall.S_IFMT == syscall.S_IFDIR, nil
}

// mkdir creates a directory at path within root.
func (dc *dirCache) mkdir(root *os.Root, path string, perm os.FileMode) error {
	dir, base := splitPath(path)
	d, err := dc.openDir(root, dir)
	if err != nil {
		return err
	}
	if err := unix.Mkdirat(int(d.Fd()), base, uint32(perm)); err != nil {
		return &os.PathError{Op: "mkdirat", Path: path, Err: err}
	}
	return nil
}

// lchown sets ownership of path without following symlinks.
func (dc *dirCache) lchown(root *os.Root, path string, uid, gid int) error {
	dir, base := splitPath(path)
	d, err := dc.openDir(root, dir)
	if err != nil {
		return err
	}
	if err := unix.Fchownat(int(d.Fd()), base, uid, gid, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return &os.PathError{Op: "fchownat", Path: path, Err: err}
	}
	return nil
}

// chmod sets the permission bits of path. It follows symlinks (i.e. acts on
// the target); callers must not invoke this for TypeSymlink entries.
func (dc *dirCache) chmod(root *os.Root, path string, mode os.FileMode) error {
	dir, base := splitPath(path)
	d, err := dc.openDir(root, dir)
	if err != nil {
		return err
	}
	if err := unix.Fchmodat(int(d.Fd()), base, fileModeToPerm(mode), 0); err != nil {
		return &os.PathError{Op: "fchmodat", Path: path, Err: err}
	}
	return nil
}

// chtimes sets access and modification times of path, following symlinks.
// Callers must not invoke this for TypeSymlink entries.
func (dc *dirCache) chtimes(root *os.Root, path string, atime, mtime time.Time) error {
	dir, base := splitPath(path)
	d, err := dc.openDir(root, dir)
	if err != nil {
		return err
	}
	utimes := [2]unix.Timespec{timeToTimespec(atime), timeToTimespec(mtime)}
	if err := unix.UtimesNanoAt(int(d.Fd()), base, utimes[0:], 0); err != nil {
		return &os.PathError{Op: "utimensat", Path: path, Err: err}
	}
	return nil
}

// lchtimes sets access and modification times of a symlink at path without
// following the symlink.
func (dc *dirCache) lchtimes(root *os.Root, path string, atime, mtime time.Time) error {
	dir, base := splitPath(path)
	d, err := dc.openDir(root, dir)
	if err != nil {
		return err
	}
	utimes := [2]unix.Timespec{timeToTimespec(atime), timeToTimespec(mtime)}
	if err := unix.UtimesNanoAt(int(d.Fd()), base, utimes[0:], unix.AT_SYMLINK_NOFOLLOW); err != nil && err != unix.ENOSYS {
		return &os.PathError{Op: "utimensat", Path: path, Err: err}
	}
	return nil
}

// fileModeToPerm converts an os.FileMode to the Unix permission and
// special-bit mask expected by fchmodat(2).
func fileModeToPerm(m os.FileMode) uint32 {
	mode := uint32(m.Perm())
	if m&os.ModeSetuid != 0 {
		mode |= syscall.S_ISUID
	}
	if m&os.ModeSetgid != 0 {
		mode |= syscall.S_ISGID
	}
	if m&os.ModeSticky != 0 {
		mode |= syscall.S_ISVTX
	}
	return mode
}
