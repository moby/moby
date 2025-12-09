//go:build !tinygo

package sysfs

import (
	"io/fs"
	"os"
	"path"

	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
)

// Link implements the same method as documented on sys.FS
func (d *dirFS) Link(oldName, newName string) experimentalsys.Errno {
	err := os.Link(d.join(oldName), d.join(newName))
	return experimentalsys.UnwrapOSError(err)
}

// Unlink implements the same method as documented on sys.FS
func (d *dirFS) Unlink(path string) (err experimentalsys.Errno) {
	return unlink(d.join(path))
}

// Rename implements the same method as documented on sys.FS
func (d *dirFS) Rename(from, to string) experimentalsys.Errno {
	from, to = d.join(from), d.join(to)
	return rename(from, to)
}

// Chmod implements the same method as documented on sys.FS
func (d *dirFS) Chmod(path string, perm fs.FileMode) experimentalsys.Errno {
	err := os.Chmod(d.join(path), perm)
	return experimentalsys.UnwrapOSError(err)
}

// Symlink implements the same method as documented on sys.FS
func (d *dirFS) Symlink(oldName, link string) experimentalsys.Errno {
	// Creating a symlink with an absolute path string fails with a "not permitted" error.
	// https://github.com/WebAssembly/wasi-filesystem/blob/v0.2.0/path-resolution.md#symlinks
	if path.IsAbs(oldName) {
		return experimentalsys.EPERM
	}
	// Note: do not resolve `oldName` relative to this dirFS. The link result is always resolved
	// when dereference the `link` on its usage (e.g. readlink, read, etc).
	// https://github.com/bytecodealliance/cap-std/blob/v1.0.4/cap-std/src/fs/dir.rs#L404-L409
	err := os.Symlink(oldName, d.join(link))
	return experimentalsys.UnwrapOSError(err)
}
