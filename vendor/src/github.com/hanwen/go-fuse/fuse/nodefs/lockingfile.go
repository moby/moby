package nodefs

import (
	"fmt"
	"sync"
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

type lockingFile struct {
	mu   *sync.Mutex
	file File
}

// NewLockingFile serializes operations an existing File.
func NewLockingFile(mu *sync.Mutex, f File) File {
	return &lockingFile{
		mu:   mu,
		file: f,
	}
}

func (f *lockingFile) SetInode(*Inode) {
}

func (f *lockingFile) InnerFile() File {
	return f.file
}

func (f *lockingFile) String() string {
	return fmt.Sprintf("lockingFile(%s)", f.file.String())
}

func (f *lockingFile) Read(buf []byte, off int64) (fuse.ReadResult, fuse.Status) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Read(buf, off)
}

func (f *lockingFile) Write(data []byte, off int64) (uint32, fuse.Status) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Write(data, off)
}

func (f *lockingFile) Flush() fuse.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Flush()
}

func (f *lockingFile) Release() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.file.Release()
}

func (f *lockingFile) GetAttr(a *fuse.Attr) fuse.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.GetAttr(a)
}

func (f *lockingFile) Fsync(flags int) (code fuse.Status) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Fsync(flags)
}

func (f *lockingFile) Utimens(atime *time.Time, mtime *time.Time) fuse.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Utimens(atime, mtime)
}

func (f *lockingFile) Truncate(size uint64) fuse.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Truncate(size)
}

func (f *lockingFile) Chown(uid uint32, gid uint32) fuse.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Chown(uid, gid)
}

func (f *lockingFile) Chmod(perms uint32) fuse.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Chmod(perms)
}

func (f *lockingFile) Allocate(off uint64, size uint64, mode uint32) (code fuse.Status) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Allocate(off, size, mode)
}
