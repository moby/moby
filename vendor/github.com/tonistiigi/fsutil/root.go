package fsutil

import (
	gofs "io/fs"
	"os"
	"sync"
	"time"
)

type Root interface {
	Close() error
	FS() gofs.FS
	Remove(string) error
	RemoveAll(string) error
	Lstat(string) (os.FileInfo, error)
	Stat(string) (os.FileInfo, error)
	Mkdir(string, os.FileMode) error
	Symlink(string, string) error
	Link(string, string) error
	OpenRoot(string) (*os.Root, error)
	OpenFile(string, int, os.FileMode) (*os.File, error)
	Readlink(string) (string, error)
	Rename(string, string) error
	Lchown(string, int, int) error
	Chmod(string, os.FileMode) error
	Chtimes(string, time.Time, time.Time) error
	RootXattr
	RootMknod
	RootLChtimes
}

type RootXattr interface {
	LSetxattr(name, key string, value []byte, flags int) error
}

type RootMknod interface {
	Mknod(name string, mode uint32, dev int) error
}

type RootLChtimes interface {
	LChtimes(name string, mtime time.Time) error
}

type root struct {
	*os.Root

	mu     sync.Mutex
	closed bool
	rootDirState
}

func NewRoot(osroot *os.Root) Root {
	return &root{Root: osroot}
}

func (r *root) Close() error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	rootDir := r.rootDir
	r.rootDir = nil
	osroot := r.Root
	r.Root = nil
	r.mu.Unlock()

	var err error
	if rootDir != nil {
		err = rootDir.Close()
	}
	if osroot != nil {
		if err2 := osroot.Close(); err == nil {
			err = err2
		}
	}
	return err
}

var _ Root = (*root)(nil)
