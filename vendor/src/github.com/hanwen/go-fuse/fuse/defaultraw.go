package fuse

import (
	"os"
)

// NewDefaultRawFileSystem returns ENOSYS (not implemented) for all
// operations.
func NewDefaultRawFileSystem() RawFileSystem {
	return (*defaultRawFileSystem)(nil)
}

type defaultRawFileSystem struct{}

func (fs *defaultRawFileSystem) Init(*Server) {
}

func (fs *defaultRawFileSystem) String() string {
	return os.Args[0]
}

func (fs *defaultRawFileSystem) SetDebug(dbg bool) {
}

func (fs *defaultRawFileSystem) StatFs(header *InHeader, out *StatfsOut) Status {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Lookup(header *InHeader, name string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Forget(nodeID, nlookup uint64) {
}

func (fs *defaultRawFileSystem) GetAttr(input *GetAttrIn, out *AttrOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Open(input *OpenIn, out *OpenOut) (status Status) {
	return OK
}

func (fs *defaultRawFileSystem) SetAttr(input *SetAttrIn, out *AttrOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Readlink(header *InHeader) (out []byte, code Status) {
	return nil, ENOSYS
}

func (fs *defaultRawFileSystem) Mknod(input *MknodIn, name string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Mkdir(input *MkdirIn, name string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Unlink(header *InHeader, name string) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Rmdir(header *InHeader, name string) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Symlink(header *InHeader, pointedTo string, linkName string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Rename(input *RenameIn, oldName string, newName string) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Link(input *LinkIn, name string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) GetXAttrSize(header *InHeader, attr string) (size int, code Status) {
	return 0, ENOSYS
}

func (fs *defaultRawFileSystem) GetXAttrData(header *InHeader, attr string) (data []byte, code Status) {
	return nil, ENODATA
}

func (fs *defaultRawFileSystem) SetXAttr(input *SetXAttrIn, attr string, data []byte) Status {
	return ENOSYS
}

func (fs *defaultRawFileSystem) ListXAttr(header *InHeader) (data []byte, code Status) {
	return nil, ENOSYS
}

func (fs *defaultRawFileSystem) RemoveXAttr(header *InHeader, attr string) Status {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Access(input *AccessIn) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Create(input *CreateIn, name string, out *CreateOut) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) OpenDir(input *OpenIn, out *OpenOut) (status Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Read(input *ReadIn, buf []byte) (ReadResult, Status) {
	return nil, ENOSYS
}

func (fs *defaultRawFileSystem) Release(input *ReleaseIn) {
}

func (fs *defaultRawFileSystem) Write(input *WriteIn, data []byte) (written uint32, code Status) {
	return 0, ENOSYS
}

func (fs *defaultRawFileSystem) Flush(input *FlushIn) Status {
	return OK
}

func (fs *defaultRawFileSystem) Fsync(input *FsyncIn) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) ReadDir(input *ReadIn, l *DirEntryList) Status {
	return ENOSYS
}

func (fs *defaultRawFileSystem) ReadDirPlus(input *ReadIn, l *DirEntryList) Status {
	return ENOSYS
}

func (fs *defaultRawFileSystem) ReleaseDir(input *ReleaseIn) {
}

func (fs *defaultRawFileSystem) FsyncDir(input *FsyncIn) (code Status) {
	return ENOSYS
}

func (fs *defaultRawFileSystem) Fallocate(in *FallocateIn) (code Status) {
	return ENOSYS
}
