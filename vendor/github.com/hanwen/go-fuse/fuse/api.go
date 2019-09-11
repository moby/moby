// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fuse provides APIs to implement filesystems in
// userspace in terms of raw FUSE protocol.
//
// A filesystem is implemented by implementing its server that provides a
// RawFileSystem interface. Typically the server embeds
// NewDefaultRawFileSystem() and implements only subset of filesystem methods:
//
//	type MyFS struct {
//		fuse.RawFileSystem
//		...
//	}
//
//	func NewMyFS() *MyFS {
//		return &MyFS{
//			RawFileSystem: fuse.NewDefaultRawFileSystem(),
//			...
//		}
//	}
//
//	// Mkdir implements "mkdir" request handler.
//	//
//	// For other requests - not explicitly implemented by MyFS - ENOSYS
//	// will be typically returned to client.
//	func (fs *MyFS) Mkdir(...) {
//		...
//	}
//
// Then the filesystem can be mounted and served to a client (typically OS
// kernel) by creating Server:
//
//	fs := NewMyFS() // implements RawFileSystem
//	fssrv, err := fuse.NewServer(fs, mountpoint, &fuse.MountOptions{...})
//	if err != nil {
//		...
//	}
//
// and letting the server do its work:
//
//	// either synchronously - .Serve() blocks until the filesystem is unmounted.
//	fssrv.Serve()
//
//	// or in the background - .Serve() is spawned in another goroutine, but
//	// before interacting with fssrv from current context we have to wait
//	// until the filesystem mounting is complete.
//	go fssrv.Serve()
//	err = fssrv.WaitMount()
//	if err != nil {
//		...
//	}
//
// The server will serve clients by dispatching their requests to the
// filesystem implementation and conveying responses back. For example "mkdir"
// FUSE request dispatches to call
//
//	fs.Mkdir(*MkdirIn, ..., *EntryOut)
//
// "stat" to call
//
//	fs.GetAttr(*GetAttrIn, *AttrOut)
//
// etc. Please refer to RawFileSystem documentation for details.
//
// Typically, each call of the API happens in its own
// goroutine, so take care to make the file system thread-safe.
//
//
// Higher level interfaces
//
// As said above this packages provides way to implement filesystems in terms of
// raw FUSE protocol. Additionally packages nodefs and pathfs provide ways to
// implement filesystem at higher levels:
//
// Package github.com/hanwen/go-fuse/fuse/nodefs provides way to implement
// filesystems in terms of inodes. This resembles kernel's idea of what a
// filesystem looks like.
//
// Package github.com/hanwen/go-fuse/fuse/pathfs provides way to implement
// filesystems in terms of path names. Working with path names is somewhat
// easier compared to inodes, however renames can be racy. Do not use pathfs if
// you care about correctness.
package fuse

// Types for users to implement.

// The result of Read is an array of bytes, but for performance
// reasons, we can also return data as a file-descriptor/offset/size
// tuple.  If the backing store for a file is another filesystem, this
// reduces the amount of copying between the kernel and the FUSE
// server.  The ReadResult interface captures both cases.
type ReadResult interface {
	// Returns the raw bytes for the read, possibly using the
	// passed buffer. The buffer should be larger than the return
	// value from Size.
	Bytes(buf []byte) ([]byte, Status)

	// Size returns how many bytes this return value takes at most.
	Size() int

	// Done() is called after sending the data to the kernel.
	Done()
}

type MountOptions struct {
	AllowOther bool

	// Options are passed as -o string to fusermount.
	Options []string

	// Default is _DEFAULT_BACKGROUND_TASKS, 12.  This numbers
	// controls the allowed number of requests that relate to
	// async I/O.  Concurrency for synchronous I/O is not limited.
	MaxBackground int

	// Write size to use.  If 0, use default. This number is
	// capped at the kernel maximum.
	MaxWrite int

	// Max read ahead to use.  If 0, use default. This number is
	// capped at the kernel maximum.
	MaxReadAhead int

	// If IgnoreSecurityLabels is set, all security related xattr
	// requests will return NO_DATA without passing through the
	// user defined filesystem.  You should only set this if you
	// file system implements extended attributes, and you are not
	// interested in security labels.
	IgnoreSecurityLabels bool // ignoring labels should be provided as a fusermount mount option.

	// If given, use this buffer pool instead of the global one.
	Buffers BufferPool

	// If RememberInodes is set, we will never forget inodes.
	// This may be useful for NFS.
	RememberInodes bool

	// Values shown in "df -T" and friends
	// First column, "Filesystem"
	FsName string

	// Second column, "Type", will be shown as "fuse." + Name
	Name string

	// If set, wrap the file system in a single-threaded locking wrapper.
	SingleThreaded bool

	// If set, return ENOSYS for Getxattr calls, so the kernel does not issue any
	// Xattr operations at all.
	DisableXAttrs bool

	// If set, print debugging information.
	Debug bool

	// If set, ask kernel to forward file locks to FUSE. If using,
	// you must implement the GetLk/SetLk/SetLkw methods.
	EnableLocks bool
}

// RawFileSystem is an interface close to the FUSE wire protocol.
//
// Unless you really know what you are doing, you should not implement
// this, but rather the nodefs.Node or pathfs.FileSystem interfaces; the
// details of getting interactions with open files, renames, and threading
// right etc. are somewhat tricky and not very interesting.
//
// Each FUSE request results in a corresponding method called by Server.
// Several calls may be made simultaneously, because the server typically calls
// each method in separate goroutine.
//
// A null implementation is provided by NewDefaultRawFileSystem.
type RawFileSystem interface {
	String() string

	// If called, provide debug output through the log package.
	SetDebug(debug bool)

	// Lookup is called by the kernel when the VFS wants to know
	// about a file inside a directory. Many lookup calls can
	// occur in parallel, but only one call happens for each (dir,
	// name) pair.
	Lookup(header *InHeader, name string, out *EntryOut) (status Status)

	// Forget is called when the kernel discards entries from its
	// dentry cache. This happens on unmount, and when the kernel
	// is short on memory. Since it is not guaranteed to occur at
	// any moment, and since there is no return value, Forget
	// should not do I/O, as there is no channel to report back
	// I/O errors.
	Forget(nodeid, nlookup uint64)

	// Attributes.
	GetAttr(input *GetAttrIn, out *AttrOut) (code Status)
	SetAttr(input *SetAttrIn, out *AttrOut) (code Status)

	// Modifying structure.
	Mknod(input *MknodIn, name string, out *EntryOut) (code Status)
	Mkdir(input *MkdirIn, name string, out *EntryOut) (code Status)
	Unlink(header *InHeader, name string) (code Status)
	Rmdir(header *InHeader, name string) (code Status)
	Rename(input *RenameIn, oldName string, newName string) (code Status)
	Link(input *LinkIn, filename string, out *EntryOut) (code Status)

	Symlink(header *InHeader, pointedTo string, linkName string, out *EntryOut) (code Status)
	Readlink(header *InHeader) (out []byte, code Status)
	Access(input *AccessIn) (code Status)

	// Extended attributes.
	GetXAttrSize(header *InHeader, attr string) (sz int, code Status)
	GetXAttrData(header *InHeader, attr string) (data []byte, code Status)
	ListXAttr(header *InHeader) (attributes []byte, code Status)
	SetXAttr(input *SetXAttrIn, attr string, data []byte) Status
	RemoveXAttr(header *InHeader, attr string) (code Status)

	// File handling.
	Create(input *CreateIn, name string, out *CreateOut) (code Status)
	Open(input *OpenIn, out *OpenOut) (status Status)
	Read(input *ReadIn, buf []byte) (ReadResult, Status)

	// File locking
	GetLk(input *LkIn, out *LkOut) (code Status)
	SetLk(input *LkIn) (code Status)
	SetLkw(input *LkIn) (code Status)

	Release(input *ReleaseIn)
	Write(input *WriteIn, data []byte) (written uint32, code Status)
	Flush(input *FlushIn) Status
	Fsync(input *FsyncIn) (code Status)
	Fallocate(input *FallocateIn) (code Status)

	// Directory handling
	OpenDir(input *OpenIn, out *OpenOut) (status Status)
	ReadDir(input *ReadIn, out *DirEntryList) Status
	ReadDirPlus(input *ReadIn, out *DirEntryList) Status
	ReleaseDir(input *ReleaseIn)
	FsyncDir(input *FsyncIn) (code Status)

	//
	StatFs(input *InHeader, out *StatfsOut) (code Status)

	// This is called on processing the first request. The
	// filesystem implementation can use the server argument to
	// talk back to the kernel (through notify methods).
	Init(*Server)
}
