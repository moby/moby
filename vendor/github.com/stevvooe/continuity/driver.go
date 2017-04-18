package continuity

import (
	"errors"
	"os"
	"strconv"
)

// Driver provides all of the system-level functions in a common interface.
// The context should call these with full paths and should never use the `os`
// package or any other package to access resources on the filesystem. This
// mechanism let's us carefully control access to the context and maintain
// path and resource integrity. It also gives us an interface to reason about
// direct resource access.
//
// Implementations don't need to do much other than meet the interface. For
// example, it is not required to wrap os.FileInfo to return correct paths for
// the call to Name().
type Driver interface {
	Open(path string) (*os.File, error)
	Stat(path string) (os.FileInfo, error)
	Lstat(path string) (os.FileInfo, error)
	Readlink(p string) (string, error)
	Mkdir(path string, mode os.FileMode) error
	Remove(path string) error

	Link(oldname, newname string) error
	Lchmod(path string, mode os.FileMode) error
	Lchown(path, uid, gid string) error
	Symlink(oldname, newname string) error

	// TODO(aaronl): These methods might move outside the main Driver
	// interface in the future as more platforms are added.
	Mknod(path string, mode os.FileMode, major int, minor int) error
	Mkfifo(path string, mode os.FileMode) error

	// NOTE(stevvooe): We may want to actually include the path manipulation
	// functions here, as well. They have been listed below to make the
	// discovery process easier.

	// Join(path ...string) string
	// IsAbs(string) bool
	// Abs(string) (string, error)
	// Rel(base, target string) (string, error)
	// Walk(string, filepath.WalkFunc) error
}

func NewSystemDriver() (Driver, error) {
	// TODO(stevvooe): Consider having this take a "hint" path argument, which
	// would be the context root. The hint could be used to resolve required
	// filesystem support when assembling the driver to use.
	return &driver{}, nil
}

// XAttrDriver should be implemented on operation systems and filesystems that
// have xattr support for regular files and directories.
type XAttrDriver interface {
	// Getxattr returns all of the extended attributes for the file at path.
	// Typically, this takes a syscall call to Listxattr and Getxattr.
	Getxattr(path string) (map[string][]byte, error)

	// Setxattr sets all of the extended attributes on file at path, following
	// any symbolic links, if necessary. All attributes on the target are
	// replaced by the values from attr. If the operation fails to set any
	// attribute, those already applied will not be rolled back.
	Setxattr(path string, attr map[string][]byte) error
}

// LXAttrDriver should be implemented by drivers on operating systems and
// filesystems that support setting and getting extended attributes on
// symbolic links. If this is not implemented, extended attributes will be
// ignored on symbolic links.
type LXAttrDriver interface {
	// LGetxattr returns all of the extended attributes for the file at path
	// and does not follow symlinks. Typically, this takes a syscall call to
	// Llistxattr and Lgetxattr.
	LGetxattr(path string) (map[string][]byte, error)

	// LSetxattr sets all of the extended attributes on file at path, without
	// following symbolic links. All attributes on the target are replaced by
	// the values from attr. If the operation fails to set any attribute,
	// those already applied will not be rolled back.
	LSetxattr(path string, attr map[string][]byte) error
}

type DeviceInfoDriver interface {
	DeviceInfo(fi os.FileInfo) (maj uint64, min uint64, err error)
}

// driver is a simple default implementation that sends calls out to the "os"
// package. Extend the "driver" type in system-specific files to add support,
// such as xattrs, which can add support at compile time.
type driver struct{}

var _ Driver = &driver{}

func (d *driver) Open(p string) (*os.File, error) {
	return os.Open(p)
}

func (d *driver) Stat(p string) (os.FileInfo, error) {
	return os.Stat(p)
}

func (d *driver) Lstat(p string) (os.FileInfo, error) {
	return os.Lstat(p)
}

func (d *driver) Readlink(p string) (string, error) {
	return os.Readlink(p)
}

func (d *driver) Mkdir(p string, mode os.FileMode) error {
	return os.Mkdir(p, mode)
}

// Remove is used to unlink files and remove directories.
// This is following the golang os package api which
// combines the operations into a higher level Remove
// function. If explicit unlinking or directory removal
// to mirror system call is required, they should be
// split up at that time.
func (d *driver) Remove(path string) error {
	return os.Remove(path)
}

func (d *driver) Link(oldname, newname string) error {
	return os.Link(oldname, newname)
}

func (d *driver) Lchown(name, uidStr, gidStr string) error {
	uid, err := strconv.Atoi(uidStr)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(gidStr)
	if err != nil {
		return err
	}
	return os.Lchown(name, uid, gid)
}

func (d *driver) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

func (d *driver) Mknod(path string, mode os.FileMode, major, minor int) error {
	return mknod(path, mode, major, minor)
}

func (d *driver) Mkfifo(path string, mode os.FileMode) error {
	if mode&os.ModeNamedPipe == 0 {
		return errors.New("mode passed to Mkfifo does not have the named pipe bit set")
	}
	// mknod with a mode that has ModeNamedPipe set creates a fifo, not a
	// device.
	return mknod(path, mode, 0, 0)
}
