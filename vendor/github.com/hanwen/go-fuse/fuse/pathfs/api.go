// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pathfs

import (
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

// A filesystem API that uses paths rather than inodes.  A minimal
// file system should have at least a functional GetAttr method.
// Typically, each call happens in its own goroutine, so take care to
// make the file system thread-safe.
//
// NewDefaultFileSystem provides a null implementation of required
// methods.
type FileSystem interface {
	// Used for pretty printing.
	String() string

	// If called, provide debug output through the log package.
	SetDebug(debug bool)

	// Attributes.  This function is the main entry point, through
	// which FUSE discovers which files and directories exist.
	//
	// If the filesystem wants to implement hard-links, it should
	// return consistent non-zero FileInfo.Ino data.  Using
	// hardlinks incurs a performance hit.
	GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status)

	// These should update the file's ctime too.
	Chmod(name string, mode uint32, context *fuse.Context) (code fuse.Status)
	Chown(name string, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status)
	Utimens(name string, Atime *time.Time, Mtime *time.Time, context *fuse.Context) (code fuse.Status)

	Truncate(name string, size uint64, context *fuse.Context) (code fuse.Status)

	Access(name string, mode uint32, context *fuse.Context) (code fuse.Status)

	// Tree structure
	Link(oldName string, newName string, context *fuse.Context) (code fuse.Status)
	Mkdir(name string, mode uint32, context *fuse.Context) fuse.Status
	Mknod(name string, mode uint32, dev uint32, context *fuse.Context) fuse.Status
	Rename(oldName string, newName string, context *fuse.Context) (code fuse.Status)
	Rmdir(name string, context *fuse.Context) (code fuse.Status)
	Unlink(name string, context *fuse.Context) (code fuse.Status)

	// Extended attributes.
	GetXAttr(name string, attribute string, context *fuse.Context) (data []byte, code fuse.Status)
	ListXAttr(name string, context *fuse.Context) (attributes []string, code fuse.Status)
	RemoveXAttr(name string, attr string, context *fuse.Context) fuse.Status
	SetXAttr(name string, attr string, data []byte, flags int, context *fuse.Context) fuse.Status

	// Called after mount.
	OnMount(nodeFs *PathNodeFs)
	OnUnmount()

	// File handling.  If opening for writing, the file's mtime
	// should be updated too.
	Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status)
	Create(name string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, code fuse.Status)

	// Directory handling
	OpenDir(name string, context *fuse.Context) (stream []fuse.DirEntry, code fuse.Status)

	// Symlinks.
	Symlink(value string, linkName string, context *fuse.Context) (code fuse.Status)
	Readlink(name string, context *fuse.Context) (string, fuse.Status)

	StatFs(name string) *fuse.StatfsOut
}

type PathNodeFsOptions struct {
	// If ClientInodes is set, use Inode returned from GetAttr to
	// find hard-linked files.
	ClientInodes bool

	// Debug controls printing of debug information.
	Debug bool
}
