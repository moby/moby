package pathfs

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

// PrefixFileSystem adds a path prefix to incoming calls.
type prefixFileSystem struct {
	FileSystem FileSystem
	Prefix     string
}

func NewPrefixFileSystem(fs FileSystem, prefix string) FileSystem {
	return &prefixFileSystem{fs, prefix}
}

func (fs *prefixFileSystem) SetDebug(debug bool) {
	fs.FileSystem.SetDebug(debug)
}

func (fs *prefixFileSystem) prefixed(n string) string {
	return filepath.Join(fs.Prefix, n)
}

func (fs *prefixFileSystem) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	return fs.FileSystem.GetAttr(fs.prefixed(name), context)
}

func (fs *prefixFileSystem) Readlink(name string, context *fuse.Context) (string, fuse.Status) {
	return fs.FileSystem.Readlink(fs.prefixed(name), context)
}

func (fs *prefixFileSystem) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) fuse.Status {
	return fs.FileSystem.Mknod(fs.prefixed(name), mode, dev, context)
}

func (fs *prefixFileSystem) Mkdir(name string, mode uint32, context *fuse.Context) fuse.Status {
	return fs.FileSystem.Mkdir(fs.prefixed(name), mode, context)
}

func (fs *prefixFileSystem) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	return fs.FileSystem.Unlink(fs.prefixed(name), context)
}

func (fs *prefixFileSystem) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	return fs.FileSystem.Rmdir(fs.prefixed(name), context)
}

func (fs *prefixFileSystem) Symlink(value string, linkName string, context *fuse.Context) (code fuse.Status) {
	return fs.FileSystem.Symlink(value, fs.prefixed(linkName), context)
}

func (fs *prefixFileSystem) Rename(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	return fs.FileSystem.Rename(fs.prefixed(oldName), fs.prefixed(newName), context)
}

func (fs *prefixFileSystem) Link(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	return fs.FileSystem.Link(fs.prefixed(oldName), fs.prefixed(newName), context)
}

func (fs *prefixFileSystem) Chmod(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	return fs.FileSystem.Chmod(fs.prefixed(name), mode, context)
}

func (fs *prefixFileSystem) Chown(name string, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	return fs.FileSystem.Chown(fs.prefixed(name), uid, gid, context)
}

func (fs *prefixFileSystem) Truncate(name string, offset uint64, context *fuse.Context) (code fuse.Status) {
	return fs.FileSystem.Truncate(fs.prefixed(name), offset, context)
}

func (fs *prefixFileSystem) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	return fs.FileSystem.Open(fs.prefixed(name), flags, context)
}

func (fs *prefixFileSystem) OpenDir(name string, context *fuse.Context) (stream []fuse.DirEntry, status fuse.Status) {
	return fs.FileSystem.OpenDir(fs.prefixed(name), context)
}

func (fs *prefixFileSystem) OnMount(nodeFs *PathNodeFs) {
	fs.FileSystem.OnMount(nodeFs)
}

func (fs *prefixFileSystem) OnUnmount() {
	fs.FileSystem.OnUnmount()
}

func (fs *prefixFileSystem) Access(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	return fs.FileSystem.Access(fs.prefixed(name), mode, context)
}

func (fs *prefixFileSystem) Create(name string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	return fs.FileSystem.Create(fs.prefixed(name), flags, mode, context)
}

func (fs *prefixFileSystem) Utimens(name string, Atime *time.Time, Mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	return fs.FileSystem.Utimens(fs.prefixed(name), Atime, Mtime, context)
}

func (fs *prefixFileSystem) GetXAttr(name string, attr string, context *fuse.Context) ([]byte, fuse.Status) {
	return fs.FileSystem.GetXAttr(fs.prefixed(name), attr, context)
}

func (fs *prefixFileSystem) SetXAttr(name string, attr string, data []byte, flags int, context *fuse.Context) fuse.Status {
	return fs.FileSystem.SetXAttr(fs.prefixed(name), attr, data, flags, context)
}

func (fs *prefixFileSystem) ListXAttr(name string, context *fuse.Context) ([]string, fuse.Status) {
	return fs.FileSystem.ListXAttr(fs.prefixed(name), context)
}

func (fs *prefixFileSystem) RemoveXAttr(name string, attr string, context *fuse.Context) fuse.Status {
	return fs.FileSystem.RemoveXAttr(fs.prefixed(name), attr, context)
}

func (fs *prefixFileSystem) String() string {
	return fmt.Sprintf("prefixFileSystem(%s,%s)", fs.FileSystem.String(), fs.Prefix)
}

func (fs *prefixFileSystem) StatFs(name string) *fuse.StatfsOut {
	return fs.FileSystem.StatFs(fs.prefixed(name))
}
