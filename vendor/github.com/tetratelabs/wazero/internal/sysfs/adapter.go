package sysfs

import (
	"fmt"
	"io/fs"
	"path"

	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/sys"
)

type AdaptFS struct {
	FS fs.FS
}

// String implements fmt.Stringer
func (a *AdaptFS) String() string {
	return fmt.Sprintf("%v", a.FS)
}

// OpenFile implements the same method as documented on sys.FS
func (a *AdaptFS) OpenFile(path string, flag experimentalsys.Oflag, perm fs.FileMode) (experimentalsys.File, experimentalsys.Errno) {
	return OpenFSFile(a.FS, cleanPath(path), flag, perm)
}

// Lstat implements the same method as documented on sys.FS
func (a *AdaptFS) Lstat(path string) (sys.Stat_t, experimentalsys.Errno) {
	// At this time, we make the assumption sys.FS instances do not support
	// symbolic links, therefore Lstat is the same as Stat. This is obviously
	// not true, but until FS.FS has a solid story for how to handle symlinks,
	// we are better off not making a decision that would be difficult to
	// revert later on.
	//
	// For further discussions on the topic, see:
	// https://github.com/golang/go/issues/49580
	return a.Stat(path)
}

// Stat implements the same method as documented on sys.FS
func (a *AdaptFS) Stat(path string) (sys.Stat_t, experimentalsys.Errno) {
	f, errno := a.OpenFile(path, experimentalsys.O_RDONLY, 0)
	if errno != 0 {
		return sys.Stat_t{}, errno
	}
	defer f.Close()
	return f.Stat()
}

// Readlink implements the same method as documented on sys.FS
func (a *AdaptFS) Readlink(string) (string, experimentalsys.Errno) {
	return "", experimentalsys.ENOSYS
}

// Mkdir implements the same method as documented on sys.FS
func (a *AdaptFS) Mkdir(string, fs.FileMode) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}

// Chmod implements the same method as documented on sys.FS
func (a *AdaptFS) Chmod(string, fs.FileMode) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}

// Rename implements the same method as documented on sys.FS
func (a *AdaptFS) Rename(string, string) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}

// Rmdir implements the same method as documented on sys.FS
func (a *AdaptFS) Rmdir(string) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}

// Link implements the same method as documented on sys.FS
func (a *AdaptFS) Link(string, string) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}

// Symlink implements the same method as documented on sys.FS
func (a *AdaptFS) Symlink(string, string) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}

// Unlink implements the same method as documented on sys.FS
func (a *AdaptFS) Unlink(string) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}

// Utimens implements the same method as documented on sys.FS
func (a *AdaptFS) Utimens(string, int64, int64) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}

func cleanPath(name string) string {
	if len(name) == 0 {
		return name
	}
	// fs.ValidFile cannot be rooted (start with '/')
	cleaned := name
	if name[0] == '/' {
		cleaned = name[1:]
	}
	cleaned = path.Clean(cleaned) // e.g. "sub/." -> "sub"
	return cleaned
}
