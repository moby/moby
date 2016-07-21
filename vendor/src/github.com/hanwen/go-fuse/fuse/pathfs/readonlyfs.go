package pathfs

import (
	"fmt"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

// NewReadonlyFileSystem returns a wrapper that only exposes read-only
// operations.
func NewReadonlyFileSystem(fs FileSystem) FileSystem {
	return &readonlyFileSystem{fs}
}

type readonlyFileSystem struct {
	FileSystem
}

func (fs *readonlyFileSystem) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	return fs.FileSystem.GetAttr(name, context)
}

func (fs *readonlyFileSystem) Readlink(name string, context *fuse.Context) (string, fuse.Status) {
	return fs.FileSystem.Readlink(name, context)
}

func (fs *readonlyFileSystem) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) fuse.Status {
	return fuse.EPERM
}

func (fs *readonlyFileSystem) Mkdir(name string, mode uint32, context *fuse.Context) fuse.Status {
	return fuse.EPERM
}

func (fs *readonlyFileSystem) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	return fuse.EPERM
}

func (fs *readonlyFileSystem) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	return fuse.EPERM
}

func (fs *readonlyFileSystem) Symlink(value string, linkName string, context *fuse.Context) (code fuse.Status) {
	return fuse.EPERM
}

func (fs *readonlyFileSystem) Rename(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	return fuse.EPERM
}

func (fs *readonlyFileSystem) Link(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	return fuse.EPERM
}

func (fs *readonlyFileSystem) Chmod(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.EPERM
}

func (fs *readonlyFileSystem) Chown(name string, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.EPERM
}

func (fs *readonlyFileSystem) Truncate(name string, offset uint64, context *fuse.Context) (code fuse.Status) {
	return fuse.EPERM
}

func (fs *readonlyFileSystem) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	if flags&fuse.O_ANYWRITE != 0 {
		return nil, fuse.EPERM
	}
	file, code = fs.FileSystem.Open(name, flags, context)
	return nodefs.NewReadOnlyFile(file), code
}

func (fs *readonlyFileSystem) OpenDir(name string, context *fuse.Context) (stream []fuse.DirEntry, status fuse.Status) {
	return fs.FileSystem.OpenDir(name, context)
}

func (fs *readonlyFileSystem) OnMount(nodeFs *PathNodeFs) {
	fs.FileSystem.OnMount(nodeFs)
}

func (fs *readonlyFileSystem) OnUnmount() {
	fs.FileSystem.OnUnmount()
}

func (fs *readonlyFileSystem) String() string {
	return fmt.Sprintf("readonlyFileSystem(%v)", fs.FileSystem)
}

func (fs *readonlyFileSystem) Access(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	return fs.FileSystem.Access(name, mode, context)
}

func (fs *readonlyFileSystem) Create(name string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	return nil, fuse.EPERM
}

func (fs *readonlyFileSystem) Utimens(name string, atime *time.Time, ctime *time.Time, context *fuse.Context) (code fuse.Status) {
	return fuse.EPERM
}

func (fs *readonlyFileSystem) GetXAttr(name string, attr string, context *fuse.Context) ([]byte, fuse.Status) {
	return fs.FileSystem.GetXAttr(name, attr, context)
}

func (fs *readonlyFileSystem) SetXAttr(name string, attr string, data []byte, flags int, context *fuse.Context) fuse.Status {
	return fuse.EPERM
}

func (fs *readonlyFileSystem) ListXAttr(name string, context *fuse.Context) ([]string, fuse.Status) {
	return fs.FileSystem.ListXAttr(name, context)
}

func (fs *readonlyFileSystem) RemoveXAttr(name string, attr string, context *fuse.Context) fuse.Status {
	return fuse.EPERM
}
