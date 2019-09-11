// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pathfs

import (
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

// NewDefaultFileSystem creates a filesystem that responds ENOSYS for
// all methods
func NewDefaultFileSystem() FileSystem {
	return (*defaultFileSystem)(nil)
}

// defaultFileSystem implements a FileSystem that returns ENOSYS for every operation.
type defaultFileSystem struct{}

func (fs *defaultFileSystem) SetDebug(debug bool) {}

// defaultFileSystem
func (fs *defaultFileSystem) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	return nil, fuse.ENOSYS
}

func (fs *defaultFileSystem) GetXAttr(name string, attr string, context *fuse.Context) ([]byte, fuse.Status) {
	return nil, fuse.ENOATTR
}

func (fs *defaultFileSystem) SetXAttr(name string, attr string, data []byte, flags int, context *fuse.Context) fuse.Status {
	return fuse.ENOSYS
}

func (fs *defaultFileSystem) ListXAttr(name string, context *fuse.Context) ([]string, fuse.Status) {
	return nil, fuse.ENOSYS
}

func (fs *defaultFileSystem) RemoveXAttr(name string, attr string, context *fuse.Context) fuse.Status {
	return fuse.ENOSYS
}

func (fs *defaultFileSystem) Readlink(name string, context *fuse.Context) (string, fuse.Status) {
	return "", fuse.ENOSYS
}

func (fs *defaultFileSystem) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) fuse.Status {
	return fuse.ENOSYS
}

func (fs *defaultFileSystem) Mkdir(name string, mode uint32, context *fuse.Context) fuse.Status {
	return fuse.ENOSYS
}

func (fs *defaultFileSystem) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (fs *defaultFileSystem) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (fs *defaultFileSystem) Symlink(value string, linkName string, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (fs *defaultFileSystem) Rename(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (fs *defaultFileSystem) Link(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (fs *defaultFileSystem) Chmod(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (fs *defaultFileSystem) Chown(name string, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (fs *defaultFileSystem) Truncate(name string, offset uint64, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (fs *defaultFileSystem) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	return nil, fuse.ENOSYS
}

func (fs *defaultFileSystem) OpenDir(name string, context *fuse.Context) (stream []fuse.DirEntry, status fuse.Status) {
	return nil, fuse.ENOSYS
}

func (fs *defaultFileSystem) OnMount(nodeFs *PathNodeFs) {
}

func (fs *defaultFileSystem) OnUnmount() {
}

func (fs *defaultFileSystem) Access(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (fs *defaultFileSystem) Create(name string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	return nil, fuse.ENOSYS
}

func (fs *defaultFileSystem) Utimens(name string, Atime *time.Time, Mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (fs *defaultFileSystem) String() string {
	return "defaultFileSystem"
}

func (fs *defaultFileSystem) StatFs(name string) *fuse.StatfsOut {
	return nil
}
