package nodefs

import (
	"fmt"
	"os"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/fuse"
)

// DataFile is for implementing read-only filesystems.  This
// assumes we already have the data in memory.
type dataFile struct {
	data []byte

	File
}

func (f *dataFile) String() string {
	l := len(f.data)
	if l > 10 {
		l = 10
	}

	return fmt.Sprintf("dataFile(%x)", f.data[:l])
}

func (f *dataFile) GetAttr(out *fuse.Attr) fuse.Status {
	out.Mode = fuse.S_IFREG | 0644
	out.Size = uint64(len(f.data))
	return fuse.OK
}

func NewDataFile(data []byte) File {
	f := new(dataFile)
	f.data = data
	f.File = NewDefaultFile()
	return f
}

func (f *dataFile) Read(buf []byte, off int64) (res fuse.ReadResult, code fuse.Status) {
	end := int(off) + int(len(buf))
	if end > len(f.data) {
		end = len(f.data)
	}

	return fuse.ReadResultData(f.data[off:end]), fuse.OK
}

type devNullFile struct {
	File
}

// NewDevNullFile returns a file that accepts any write, and always
// returns EOF for reads.
func NewDevNullFile() File {
	return &devNullFile{
		File: NewDefaultFile(),
	}
}

func (f *devNullFile) Allocate(off uint64, size uint64, mode uint32) (code fuse.Status) {
	return fuse.OK
}

func (f *devNullFile) String() string {
	return "devNullFile"
}

func (f *devNullFile) Read(buf []byte, off int64) (fuse.ReadResult, fuse.Status) {
	return fuse.ReadResultData(nil), fuse.OK
}

func (f *devNullFile) Write(content []byte, off int64) (uint32, fuse.Status) {
	return uint32(len(content)), fuse.OK
}

func (f *devNullFile) Flush() fuse.Status {
	return fuse.OK
}

func (f *devNullFile) Fsync(flags int) (code fuse.Status) {
	return fuse.OK
}

func (f *devNullFile) Truncate(size uint64) (code fuse.Status) {
	return fuse.OK
}

////////////////

// LoopbackFile delegates all operations back to an underlying os.File.
func NewLoopbackFile(f *os.File) File {
	return &loopbackFile{File: f}
}

type loopbackFile struct {
	File *os.File

	// os.File is not threadsafe. Although fd themselves are
	// constant during the lifetime of an open file, the OS may
	// reuse the fd number after it is closed. When open races
	// with another close, they may lead to confusion as which
	// file gets written in the end.
	lock sync.Mutex
}

func (f *loopbackFile) InnerFile() File {
	return nil
}

func (f *loopbackFile) SetInode(n *Inode) {
}

func (f *loopbackFile) String() string {
	return fmt.Sprintf("loopbackFile(%s)", f.File.Name())
}

func (f *loopbackFile) Read(buf []byte, off int64) (res fuse.ReadResult, code fuse.Status) {
	f.lock.Lock()
	// This is not racy by virtue of the kernel properly
	// synchronizing the open/write/close.
	r := fuse.ReadResultFd(f.File.Fd(), off, len(buf))
	f.lock.Unlock()
	return r, fuse.OK
}

func (f *loopbackFile) Write(data []byte, off int64) (uint32, fuse.Status) {
	f.lock.Lock()
	n, err := f.File.WriteAt(data, off)
	f.lock.Unlock()
	return uint32(n), fuse.ToStatus(err)
}

func (f *loopbackFile) Release() {
	f.lock.Lock()
	f.File.Close()
	f.lock.Unlock()
}

func (f *loopbackFile) Flush() fuse.Status {
	f.lock.Lock()

	// Since Flush() may be called for each dup'd fd, we don't
	// want to really close the file, we just want to flush. This
	// is achieved by closing a dup'd fd.
	newFd, err := syscall.Dup(int(f.File.Fd()))
	f.lock.Unlock()

	if err != nil {
		return fuse.ToStatus(err)
	}
	err = syscall.Close(newFd)
	return fuse.ToStatus(err)
}

func (f *loopbackFile) Fsync(flags int) (code fuse.Status) {
	f.lock.Lock()
	r := fuse.ToStatus(syscall.Fsync(int(f.File.Fd())))
	f.lock.Unlock()

	return r
}

func (f *loopbackFile) Truncate(size uint64) fuse.Status {
	f.lock.Lock()
	r := fuse.ToStatus(syscall.Ftruncate(int(f.File.Fd()), int64(size)))
	f.lock.Unlock()

	return r
}

func (f *loopbackFile) Chmod(mode uint32) fuse.Status {
	f.lock.Lock()
	r := fuse.ToStatus(f.File.Chmod(os.FileMode(mode)))
	f.lock.Unlock()

	return r
}

func (f *loopbackFile) Chown(uid uint32, gid uint32) fuse.Status {
	f.lock.Lock()
	r := fuse.ToStatus(f.File.Chown(int(uid), int(gid)))
	f.lock.Unlock()

	return r
}

func (f *loopbackFile) GetAttr(a *fuse.Attr) fuse.Status {
	st := syscall.Stat_t{}
	f.lock.Lock()
	err := syscall.Fstat(int(f.File.Fd()), &st)
	f.lock.Unlock()
	if err != nil {
		return fuse.ToStatus(err)
	}
	a.FromStat(&st)

	return fuse.OK
}

// Utimens implemented in files_linux.go

// Allocate implemented in files_linux.go

////////////////////////////////////////////////////////////////

// NewReadOnlyFile wraps a File so all read/write operations are
// denied.
func NewReadOnlyFile(f File) File {
	return &readOnlyFile{File: f}
}

type readOnlyFile struct {
	File
}

func (f *readOnlyFile) InnerFile() File {
	return f.File
}

func (f *readOnlyFile) String() string {
	return fmt.Sprintf("readOnlyFile(%s)", f.File.String())
}

func (f *readOnlyFile) Write(data []byte, off int64) (uint32, fuse.Status) {
	return 0, fuse.EPERM
}

func (f *readOnlyFile) Fsync(flag int) (code fuse.Status) {
	return fuse.OK
}

func (f *readOnlyFile) Truncate(size uint64) fuse.Status {
	return fuse.EPERM
}

func (f *readOnlyFile) Chmod(mode uint32) fuse.Status {
	return fuse.EPERM
}

func (f *readOnlyFile) Chown(uid uint32, gid uint32) fuse.Status {
	return fuse.EPERM
}

func (f *readOnlyFile) Allocate(off uint64, sz uint64, mode uint32) fuse.Status {
	return fuse.EPERM
}
