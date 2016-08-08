package pathfs

import (
	"sync"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

type lockingFileSystem struct {
	// Should be public so people reusing can access the wrapped
	// FS.
	FS   FileSystem
	lock sync.Mutex
}

// NewLockingFileSystem is a wrapper that makes a FileSystem
// threadsafe by serializing each operation.
func NewLockingFileSystem(pfs FileSystem) FileSystem {
	l := new(lockingFileSystem)
	l.FS = pfs
	return l
}

func (fs *lockingFileSystem) String() string {
	defer fs.locked()()
	return fs.FS.String()
}

func (fs *lockingFileSystem) SetDebug(debug bool) {
	defer fs.locked()()
	fs.FS.SetDebug(debug)
}

func (fs *lockingFileSystem) StatFs(name string) *fuse.StatfsOut {
	defer fs.locked()()
	return fs.FS.StatFs(name)
}

func (fs *lockingFileSystem) locked() func() {
	fs.lock.Lock()
	return func() { fs.lock.Unlock() }
}

func (fs *lockingFileSystem) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	defer fs.locked()()
	return fs.FS.GetAttr(name, context)
}

func (fs *lockingFileSystem) Readlink(name string, context *fuse.Context) (string, fuse.Status) {
	defer fs.locked()()
	return fs.FS.Readlink(name, context)
}

func (fs *lockingFileSystem) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) fuse.Status {
	defer fs.locked()()
	return fs.FS.Mknod(name, mode, dev, context)
}

func (fs *lockingFileSystem) Mkdir(name string, mode uint32, context *fuse.Context) fuse.Status {
	defer fs.locked()()
	return fs.FS.Mkdir(name, mode, context)
}

func (fs *lockingFileSystem) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	defer fs.locked()()
	return fs.FS.Unlink(name, context)
}

func (fs *lockingFileSystem) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	defer fs.locked()()
	return fs.FS.Rmdir(name, context)
}

func (fs *lockingFileSystem) Symlink(value string, linkName string, context *fuse.Context) (code fuse.Status) {
	defer fs.locked()()
	return fs.FS.Symlink(value, linkName, context)
}

func (fs *lockingFileSystem) Rename(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	defer fs.locked()()
	return fs.FS.Rename(oldName, newName, context)
}

func (fs *lockingFileSystem) Link(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	defer fs.locked()()
	return fs.FS.Link(oldName, newName, context)
}

func (fs *lockingFileSystem) Chmod(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	defer fs.locked()()
	return fs.FS.Chmod(name, mode, context)
}

func (fs *lockingFileSystem) Chown(name string, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	defer fs.locked()()
	return fs.FS.Chown(name, uid, gid, context)
}

func (fs *lockingFileSystem) Truncate(name string, offset uint64, context *fuse.Context) (code fuse.Status) {
	defer fs.locked()()
	return fs.FS.Truncate(name, offset, context)
}

func (fs *lockingFileSystem) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	file, code = fs.FS.Open(name, flags, context)
	file = nodefs.NewLockingFile(&fs.lock, file)
	return
}

func (fs *lockingFileSystem) OpenDir(name string, context *fuse.Context) (stream []fuse.DirEntry, status fuse.Status) {
	defer fs.locked()()
	return fs.FS.OpenDir(name, context)
}

func (fs *lockingFileSystem) OnMount(nodeFs *PathNodeFs) {
	defer fs.locked()()
	fs.FS.OnMount(nodeFs)
}

func (fs *lockingFileSystem) OnUnmount() {
	defer fs.locked()()
	fs.FS.OnUnmount()
}

func (fs *lockingFileSystem) Access(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	defer fs.locked()()
	return fs.FS.Access(name, mode, context)
}

func (fs *lockingFileSystem) Create(name string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	defer fs.locked()()
	file, code = fs.FS.Create(name, flags, mode, context)

	file = nodefs.NewLockingFile(&fs.lock, file)
	return file, code
}

func (fs *lockingFileSystem) Utimens(name string, Atime *time.Time, Mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	defer fs.locked()()
	return fs.FS.Utimens(name, Atime, Mtime, context)
}

func (fs *lockingFileSystem) GetXAttr(name string, attr string, context *fuse.Context) ([]byte, fuse.Status) {
	defer fs.locked()()
	return fs.FS.GetXAttr(name, attr, context)
}

func (fs *lockingFileSystem) SetXAttr(name string, attr string, data []byte, flags int, context *fuse.Context) fuse.Status {
	defer fs.locked()()
	return fs.FS.SetXAttr(name, attr, data, flags, context)
}

func (fs *lockingFileSystem) ListXAttr(name string, context *fuse.Context) ([]string, fuse.Status) {
	defer fs.locked()()
	return fs.FS.ListXAttr(name, context)
}

func (fs *lockingFileSystem) RemoveXAttr(name string, attr string, context *fuse.Context) fuse.Status {
	defer fs.locked()()
	return fs.FS.RemoveXAttr(name, attr, context)
}
