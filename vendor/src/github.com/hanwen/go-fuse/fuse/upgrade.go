package fuse

import (
	"fmt"
)

// NewRawFileSystem adds the methods missing for implementing a
// RawFileSystem to any object.
func NewRawFileSystem(fs interface{}) RawFileSystem {
	return &wrappingFS{fs}
}

type wrappingFS struct {
	fs interface{}
}

func (fs *wrappingFS) Init(srv *Server) {
	if s, ok := fs.fs.(interface {
		Init(*Server)
	}); ok {
		s.Init(srv)
	}
}

func (fs *wrappingFS) String() string {
	return fmt.Sprintf("%v", fs.fs)
}

func (fs *wrappingFS) SetDebug(dbg bool) {
	if s, ok := fs.fs.(interface {
		SetDebug(bool)
	}); ok {
		s.SetDebug(dbg)
	}
}

func (fs *wrappingFS) StatFs(header *InHeader, out *StatfsOut) Status {
	if s, ok := fs.fs.(interface {
		StatFs(header *InHeader, out *StatfsOut) Status
	}); ok {
		return s.StatFs(header, out)
	}
	return ENOSYS
}

func (fs *wrappingFS) Lookup(header *InHeader, name string, out *EntryOut) (code Status) {
	if s, ok := fs.fs.(interface {
		Lookup(header *InHeader, name string, out *EntryOut) (code Status)
	}); ok {
		return s.Lookup(header, name, out)
	}
	return ENOSYS
}

func (fs *wrappingFS) Forget(nodeID, nlookup uint64) {
	if s, ok := fs.fs.(interface {
		Forget(nodeID, nlookup uint64)
	}); ok {
		s.Forget(nodeID, nlookup)
	}
}

func (fs *wrappingFS) GetAttr(input *GetAttrIn, out *AttrOut) (code Status) {
	if s, ok := fs.fs.(interface {
		GetAttr(input *GetAttrIn, out *AttrOut) (code Status)
	}); ok {
		return s.GetAttr(input, out)
	}
	return ENOSYS
}

func (fs *wrappingFS) Open(input *OpenIn, out *OpenOut) (status Status) {
	if s, ok := fs.fs.(interface {
		Open(input *OpenIn, out *OpenOut) (status Status)
	}); ok {
		return s.Open(input, out)
	}
	return ENOSYS
}

func (fs *wrappingFS) SetAttr(input *SetAttrIn, out *AttrOut) (code Status) {
	if s, ok := fs.fs.(interface {
		SetAttr(input *SetAttrIn, out *AttrOut) (code Status)
	}); ok {
		return s.SetAttr(input, out)
	}
	return ENOSYS
}

func (fs *wrappingFS) Readlink(header *InHeader) (out []byte, code Status) {
	if s, ok := fs.fs.(interface {
		Readlink(header *InHeader) (out []byte, code Status)
	}); ok {
		return s.Readlink(header)
	}
	return nil, ENOSYS
}

func (fs *wrappingFS) Mknod(input *MknodIn, name string, out *EntryOut) (code Status) {
	if s, ok := fs.fs.(interface {
		Mknod(input *MknodIn, name string, out *EntryOut) (code Status)
	}); ok {
		return s.Mknod(input, name, out)
	}
	return ENOSYS
}

func (fs *wrappingFS) Mkdir(input *MkdirIn, name string, out *EntryOut) (code Status) {
	if s, ok := fs.fs.(interface {
		Mkdir(input *MkdirIn, name string, out *EntryOut) (code Status)
	}); ok {
		return s.Mkdir(input, name, out)
	}
	return ENOSYS
}

func (fs *wrappingFS) Unlink(header *InHeader, name string) (code Status) {
	if s, ok := fs.fs.(interface {
		Unlink(header *InHeader, name string) (code Status)
	}); ok {
		return s.Unlink(header, name)
	}
	return ENOSYS
}

func (fs *wrappingFS) Rmdir(header *InHeader, name string) (code Status) {
	if s, ok := fs.fs.(interface {
		Rmdir(header *InHeader, name string) (code Status)
	}); ok {
		return s.Rmdir(header, name)
	}
	return ENOSYS
}

func (fs *wrappingFS) Symlink(header *InHeader, pointedTo string, linkName string, out *EntryOut) (code Status) {
	if s, ok := fs.fs.(interface {
		Symlink(header *InHeader, pointedTo string, linkName string, out *EntryOut) (code Status)
	}); ok {
		return s.Symlink(header, pointedTo, linkName, out)
	}
	return ENOSYS
}

func (fs *wrappingFS) Rename(input *RenameIn, oldName string, newName string) (code Status) {
	if s, ok := fs.fs.(interface {
		Rename(input *RenameIn, oldName string, newName string) (code Status)
	}); ok {
		return s.Rename(input, oldName, newName)
	}
	return ENOSYS
}

func (fs *wrappingFS) Link(input *LinkIn, name string, out *EntryOut) (code Status) {
	if s, ok := fs.fs.(interface {
		Link(input *LinkIn, name string, out *EntryOut) (code Status)
	}); ok {
		return s.Link(input, name, out)
	}
	return ENOSYS
}

func (fs *wrappingFS) GetXAttrSize(header *InHeader, attr string) (size int, code Status) {
	if s, ok := fs.fs.(interface {
		GetXAttrSize(header *InHeader, attr string) (size int, code Status)
	}); ok {
		return s.GetXAttrSize(header, attr)
	}
	return 0, ENOSYS
}

func (fs *wrappingFS) GetXAttrData(header *InHeader, attr string) (data []byte, code Status) {
	if s, ok := fs.fs.(interface {
		GetXAttrData(header *InHeader, attr string) (data []byte, code Status)
	}); ok {
		return s.GetXAttrData(header, attr)
	}
	return nil, ENOSYS
}

func (fs *wrappingFS) SetXAttr(input *SetXAttrIn, attr string, data []byte) Status {
	if s, ok := fs.fs.(interface {
		SetXAttr(input *SetXAttrIn, attr string, data []byte) Status
	}); ok {
		return s.SetXAttr(input, attr, data)
	}
	return ENOSYS
}

func (fs *wrappingFS) ListXAttr(header *InHeader) (data []byte, code Status) {
	if s, ok := fs.fs.(interface {
		ListXAttr(header *InHeader) (data []byte, code Status)
	}); ok {
		return s.ListXAttr(header)
	}
	return nil, ENOSYS
}

func (fs *wrappingFS) RemoveXAttr(header *InHeader, attr string) Status {
	if s, ok := fs.fs.(interface {
		RemoveXAttr(header *InHeader, attr string) Status
	}); ok {
		return s.RemoveXAttr(header, attr)
	}
	return ENOSYS
}

func (fs *wrappingFS) Access(input *AccessIn) (code Status) {
	if s, ok := fs.fs.(interface {
		Access(input *AccessIn) (code Status)
	}); ok {
		return s.Access(input)
	}
	return ENOSYS
}

func (fs *wrappingFS) Create(input *CreateIn, name string, out *CreateOut) (code Status) {
	if s, ok := fs.fs.(interface {
		Create(input *CreateIn, name string, out *CreateOut) (code Status)
	}); ok {
		return s.Create(input, name, out)
	}
	return ENOSYS
}

func (fs *wrappingFS) OpenDir(input *OpenIn, out *OpenOut) (status Status) {
	if s, ok := fs.fs.(interface {
		OpenDir(input *OpenIn, out *OpenOut) (status Status)
	}); ok {
		return s.OpenDir(input, out)
	}
	return ENOSYS
}

func (fs *wrappingFS) Read(input *ReadIn, buf []byte) (ReadResult, Status) {
	if s, ok := fs.fs.(interface {
		Read(input *ReadIn, buf []byte) (ReadResult, Status)
	}); ok {
		return s.Read(input, buf)
	}
	return nil, ENOSYS
}

func (fs *wrappingFS) Release(input *ReleaseIn) {
	if s, ok := fs.fs.(interface {
		Release(input *ReleaseIn)
	}); ok {
		s.Release(input)
	}
}

func (fs *wrappingFS) Write(input *WriteIn, data []byte) (written uint32, code Status) {
	if s, ok := fs.fs.(interface {
		Write(input *WriteIn, data []byte) (written uint32, code Status)
	}); ok {
		return s.Write(input, data)
	}
	return 0, ENOSYS
}

func (fs *wrappingFS) Flush(input *FlushIn) Status {
	if s, ok := fs.fs.(interface {
		Flush(input *FlushIn) Status
	}); ok {
		return s.Flush(input)
	}
	return OK
}

func (fs *wrappingFS) Fsync(input *FsyncIn) (code Status) {
	if s, ok := fs.fs.(interface {
		Fsync(input *FsyncIn) (code Status)
	}); ok {
		return s.Fsync(input)
	}
	return ENOSYS
}

func (fs *wrappingFS) ReadDir(input *ReadIn, l *DirEntryList) Status {
	if s, ok := fs.fs.(interface {
		ReadDir(input *ReadIn, l *DirEntryList) Status
	}); ok {
		return s.ReadDir(input, l)
	}
	return ENOSYS
}

func (fs *wrappingFS) ReadDirPlus(input *ReadIn, l *DirEntryList) Status {
	if s, ok := fs.fs.(interface {
		ReadDirPlus(input *ReadIn, l *DirEntryList) Status
	}); ok {
		return s.ReadDirPlus(input, l)
	}
	return ENOSYS
}

func (fs *wrappingFS) ReleaseDir(input *ReleaseIn) {
	if s, ok := fs.fs.(interface {
		ReleaseDir(input *ReleaseIn)
	}); ok {
		s.ReleaseDir(input)
	}
}

func (fs *wrappingFS) FsyncDir(input *FsyncIn) (code Status) {
	if s, ok := fs.fs.(interface {
		FsyncDir(input *FsyncIn) (code Status)
	}); ok {
		return s.FsyncDir(input)
	}
	return ENOSYS
}

func (fs *wrappingFS) Fallocate(in *FallocateIn) (code Status) {
	if s, ok := fs.fs.(interface {
		Fallocate(in *FallocateIn) (code Status)
	}); ok {
		return s.Fallocate(in)
	}
	return ENOSYS
}
