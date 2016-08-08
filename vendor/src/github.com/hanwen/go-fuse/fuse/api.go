// The fuse package provides APIs to implement filesystems in
// userspace.  Typically, each call of the API happens in its own
// goroutine, so take care to make the file system thread-safe.

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
}

// RawFileSystem is an interface close to the FUSE wire protocol.
//
// Unless you really know what you are doing, you should not implement
// this, but rather the FileSystem interface; the details of getting
// interactions with open files, renames, and threading right etc. are
// somewhat tricky and not very interesting.
//
// A null implementation is provided by NewDefaultRawFileSystem.
type RawFileSystem interface {
	String() string

	// If called, provide debug output through the log package.
	SetDebug(debug bool)

	Lookup(header *InHeader, name string, out *EntryOut) (status Status)
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
