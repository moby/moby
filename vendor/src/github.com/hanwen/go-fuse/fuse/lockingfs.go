package fuse

import (
	"fmt"
	"sync"
)

////////////////////////////////////////////////////////////////
// Locking raw FS.

type lockingRawFileSystem struct {
	RawFS RawFileSystem
	lock  sync.Mutex
}

// Returns a Wrap
func NewLockingRawFileSystem(fs RawFileSystem) RawFileSystem {
	return &lockingRawFileSystem{
		RawFS: fs,
	}
}

func (fs *lockingRawFileSystem) FS() RawFileSystem {
	return fs.RawFS
}

func (fs *lockingRawFileSystem) locked() func() {
	fs.lock.Lock()
	return func() { fs.lock.Unlock() }
}

func (fs *lockingRawFileSystem) Lookup(header *InHeader, name string, out *EntryOut) (code Status) {
	defer fs.locked()()
	return fs.RawFS.Lookup(header, name, out)
}

func (fs *lockingRawFileSystem) SetDebug(dbg bool) {
	defer fs.locked()()
	fs.RawFS.SetDebug(dbg)
}

func (fs *lockingRawFileSystem) Forget(nodeID uint64, nlookup uint64) {
	defer fs.locked()()
	fs.RawFS.Forget(nodeID, nlookup)
}

func (fs *lockingRawFileSystem) GetAttr(input *GetAttrIn, out *AttrOut) (code Status) {
	defer fs.locked()()
	return fs.RawFS.GetAttr(input, out)
}

func (fs *lockingRawFileSystem) Open(input *OpenIn, out *OpenOut) (status Status) {

	defer fs.locked()()
	return fs.RawFS.Open(input, out)
}

func (fs *lockingRawFileSystem) SetAttr(input *SetAttrIn, out *AttrOut) (code Status) {
	defer fs.locked()()
	return fs.RawFS.SetAttr(input, out)
}

func (fs *lockingRawFileSystem) Readlink(header *InHeader) (out []byte, code Status) {
	defer fs.locked()()
	return fs.RawFS.Readlink(header)
}

func (fs *lockingRawFileSystem) Mknod(input *MknodIn, name string, out *EntryOut) (code Status) {
	defer fs.locked()()
	return fs.RawFS.Mknod(input, name, out)
}

func (fs *lockingRawFileSystem) Mkdir(input *MkdirIn, name string, out *EntryOut) (code Status) {
	defer fs.locked()()
	return fs.RawFS.Mkdir(input, name, out)
}

func (fs *lockingRawFileSystem) Unlink(header *InHeader, name string) (code Status) {
	defer fs.locked()()
	return fs.RawFS.Unlink(header, name)
}

func (fs *lockingRawFileSystem) Rmdir(header *InHeader, name string) (code Status) {
	defer fs.locked()()
	return fs.RawFS.Rmdir(header, name)
}

func (fs *lockingRawFileSystem) Symlink(header *InHeader, pointedTo string, linkName string, out *EntryOut) (code Status) {
	defer fs.locked()()
	return fs.RawFS.Symlink(header, pointedTo, linkName, out)
}

func (fs *lockingRawFileSystem) Rename(input *RenameIn, oldName string, newName string) (code Status) {
	defer fs.locked()()
	return fs.RawFS.Rename(input, oldName, newName)
}

func (fs *lockingRawFileSystem) Link(input *LinkIn, name string, out *EntryOut) (code Status) {
	defer fs.locked()()
	return fs.RawFS.Link(input, name, out)
}

func (fs *lockingRawFileSystem) SetXAttr(input *SetXAttrIn, attr string, data []byte) Status {
	defer fs.locked()()
	return fs.RawFS.SetXAttr(input, attr, data)
}

func (fs *lockingRawFileSystem) GetXAttrData(header *InHeader, attr string) (data []byte, code Status) {
	defer fs.locked()()
	return fs.RawFS.GetXAttrData(header, attr)
}

func (fs *lockingRawFileSystem) GetXAttrSize(header *InHeader, attr string) (sz int, code Status) {
	defer fs.locked()()
	return fs.RawFS.GetXAttrSize(header, attr)
}

func (fs *lockingRawFileSystem) ListXAttr(header *InHeader) (data []byte, code Status) {
	defer fs.locked()()
	return fs.RawFS.ListXAttr(header)
}

func (fs *lockingRawFileSystem) RemoveXAttr(header *InHeader, attr string) Status {
	defer fs.locked()()
	return fs.RawFS.RemoveXAttr(header, attr)
}

func (fs *lockingRawFileSystem) Access(input *AccessIn) (code Status) {
	defer fs.locked()()
	return fs.RawFS.Access(input)
}

func (fs *lockingRawFileSystem) Create(input *CreateIn, name string, out *CreateOut) (code Status) {
	defer fs.locked()()
	return fs.RawFS.Create(input, name, out)
}

func (fs *lockingRawFileSystem) OpenDir(input *OpenIn, out *OpenOut) (status Status) {
	defer fs.locked()()
	return fs.RawFS.OpenDir(input, out)
}

func (fs *lockingRawFileSystem) Release(input *ReleaseIn) {
	defer fs.locked()()
	fs.RawFS.Release(input)
}

func (fs *lockingRawFileSystem) ReleaseDir(input *ReleaseIn) {
	defer fs.locked()()
	fs.RawFS.ReleaseDir(input)
}

func (fs *lockingRawFileSystem) Read(input *ReadIn, buf []byte) (ReadResult, Status) {
	defer fs.locked()()
	return fs.RawFS.Read(input, buf)
}

func (fs *lockingRawFileSystem) Write(input *WriteIn, data []byte) (written uint32, code Status) {
	defer fs.locked()()
	return fs.RawFS.Write(input, data)
}

func (fs *lockingRawFileSystem) Flush(input *FlushIn) Status {
	defer fs.locked()()
	return fs.RawFS.Flush(input)
}

func (fs *lockingRawFileSystem) Fsync(input *FsyncIn) (code Status) {
	defer fs.locked()()
	return fs.RawFS.Fsync(input)
}

func (fs *lockingRawFileSystem) ReadDir(input *ReadIn, out *DirEntryList) Status {
	defer fs.locked()()
	return fs.RawFS.ReadDir(input, out)
}

func (fs *lockingRawFileSystem) ReadDirPlus(input *ReadIn, out *DirEntryList) Status {
	defer fs.locked()()
	return fs.RawFS.ReadDirPlus(input, out)
}

func (fs *lockingRawFileSystem) FsyncDir(input *FsyncIn) (code Status) {
	defer fs.locked()()
	return fs.RawFS.FsyncDir(input)
}

func (fs *lockingRawFileSystem) Init(s *Server) {
	defer fs.locked()()
	fs.RawFS.Init(s)
}

func (fs *lockingRawFileSystem) StatFs(header *InHeader, out *StatfsOut) (code Status) {
	defer fs.locked()()
	return fs.RawFS.StatFs(header, out)
}

func (fs *lockingRawFileSystem) Fallocate(in *FallocateIn) (code Status) {
	defer fs.locked()()
	return fs.RawFS.Fallocate(in)
}

func (fs *lockingRawFileSystem) String() string {
	defer fs.locked()()
	return fmt.Sprintf("Locked(%s)", fs.RawFS.String())
}
