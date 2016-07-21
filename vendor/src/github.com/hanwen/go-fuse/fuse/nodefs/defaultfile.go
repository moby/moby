package nodefs

import (
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

type defaultFile struct{}

// NewDefaultFile returns a File instance that returns ENOSYS for
// every operation.
func NewDefaultFile() File {
	return (*defaultFile)(nil)
}

func (f *defaultFile) SetInode(*Inode) {
}

func (f *defaultFile) InnerFile() File {
	return nil
}

func (f *defaultFile) String() string {
	return "defaultFile"
}

func (f *defaultFile) Read(buf []byte, off int64) (fuse.ReadResult, fuse.Status) {
	return nil, fuse.ENOSYS
}

func (f *defaultFile) Write(data []byte, off int64) (uint32, fuse.Status) {
	return 0, fuse.ENOSYS
}

func (f *defaultFile) Flush() fuse.Status {
	return fuse.OK
}

func (f *defaultFile) Release() {

}

func (f *defaultFile) GetAttr(*fuse.Attr) fuse.Status {
	return fuse.ENOSYS
}

func (f *defaultFile) Fsync(flags int) (code fuse.Status) {
	return fuse.ENOSYS
}

func (f *defaultFile) Utimens(atime *time.Time, mtime *time.Time) fuse.Status {
	return fuse.ENOSYS
}

func (f *defaultFile) Truncate(size uint64) fuse.Status {
	return fuse.ENOSYS
}

func (f *defaultFile) Chown(uid uint32, gid uint32) fuse.Status {
	return fuse.ENOSYS
}

func (f *defaultFile) Chmod(perms uint32) fuse.Status {
	return fuse.ENOSYS
}

func (f *defaultFile) Allocate(off uint64, size uint64, mode uint32) (code fuse.Status) {
	return fuse.ENOSYS
}
