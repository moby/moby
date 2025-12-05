//go:build tinygo

package sysfs

import (
	"io/fs"

	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
)

// Link implements the same method as documented on sys.FS
func (d *dirFS) Link(oldName, newName string) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}

// Unlink implements the same method as documented on sys.FS
func (d *dirFS) Unlink(path string) (err experimentalsys.Errno) {
	return experimentalsys.ENOSYS
}

// Rename implements the same method as documented on sys.FS
func (d *dirFS) Rename(from, to string) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}

// Chmod implements the same method as documented on sys.FS
func (d *dirFS) Chmod(path string, perm fs.FileMode) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}

// Symlink implements the same method as documented on sys.FS
func (d *dirFS) Symlink(oldName, link string) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}
