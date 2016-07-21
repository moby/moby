package pathfs

import (
	"fmt"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

func (fs *loopbackFileSystem) StatFs(name string) *fuse.StatfsOut {
	s := syscall.Statfs_t{}
	err := syscall.Statfs(fs.GetPath(name), &s)
	if err == nil {
		return &fuse.StatfsOut{
			Blocks:  s.Blocks,
			Bsize:   uint32(s.Bsize),
			Bfree:   s.Bfree,
			Bavail:  s.Bavail,
			Files:   s.Files,
			Ffree:   s.Ffree,
			Frsize:  uint32(s.Frsize),
			NameLen: uint32(s.Namelen),
		}
	}
	return nil
}

func (fs *loopbackFileSystem) ListXAttr(name string, context *fuse.Context) ([]string, fuse.Status) {
	data, err := listXAttr(fs.GetPath(name))

	return data, fuse.ToStatus(err)
}

func (fs *loopbackFileSystem) RemoveXAttr(name string, attr string, context *fuse.Context) fuse.Status {
	err := sysRemovexattr(fs.GetPath(name), attr)
	return fuse.ToStatus(err)
}

func (fs *loopbackFileSystem) String() string {
	return fmt.Sprintf("LoopbackFs(%s)", fs.Root)
}

func (fs *loopbackFileSystem) GetXAttr(name string, attr string, context *fuse.Context) ([]byte, fuse.Status) {
	data := make([]byte, 1024)
	data, err := getXAttr(fs.GetPath(name), attr, data)

	return data, fuse.ToStatus(err)
}

func (fs *loopbackFileSystem) SetXAttr(name string, attr string, data []byte, flags int, context *fuse.Context) fuse.Status {
	err := syscall.Setxattr(fs.GetPath(name), attr, data, flags)
	return fuse.ToStatus(err)
}

const _UTIME_NOW = ((1 << 30) - 1)
const _UTIME_OMIT = ((1 << 30) - 2)

// Utimens - path based version of loopbackFile.Utimens()
func (fs *loopbackFileSystem) Utimens(path string, a *time.Time, m *time.Time, context *fuse.Context) (code fuse.Status) {
	var ts [2]syscall.Timespec

	if a == nil {
		ts[0].Nsec = _UTIME_OMIT
	} else {
		ts[0] = syscall.NsecToTimespec(a.UnixNano())
		ts[0].Nsec = 0
	}

	if m == nil {
		ts[1].Nsec = _UTIME_OMIT
	} else {
		ts[1] = syscall.NsecToTimespec(m.UnixNano())
		ts[1].Nsec = 0
	}

	err := sysUtimensat(0, fs.GetPath(path), &ts, _AT_SYMLINK_NOFOLLOW)
	return fuse.ToStatus(err)
}
