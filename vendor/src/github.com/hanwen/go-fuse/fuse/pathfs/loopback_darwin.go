package pathfs

import (
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

func (fs *loopbackFileSystem) StatFs(name string) *fuse.StatfsOut {
	s := syscall.Statfs_t{}
	err := syscall.Statfs(fs.GetPath(name), &s)
	if err == nil {
		return &fuse.StatfsOut{
			Blocks: s.Blocks,
			Bsize:  uint32(s.Bsize),
			Bfree:  s.Bfree,
			Bavail: s.Bavail,
			Files:  s.Files,
			Ffree:  s.Ffree,
		}
	}
	return nil
}

const _UTIME_NOW = ((1 << 30) - 1)
const _UTIME_OMIT = ((1 << 30) - 2)

// timeToTimeval - Convert time.Time to syscall.Timeval
//
// Note: This does not use syscall.NsecToTimespec because
// that does not work properly for times before 1970,
// see https://github.com/golang/go/issues/12777
func timeToTimeval(t *time.Time) syscall.Timeval {
	var tv syscall.Timeval
	tv.Usec = int32(t.Nanosecond() / 1000)
	tv.Sec = t.Unix()
	return tv
}

// OSX does not have the utimensat syscall neded to implement this properly.
// We do our best to emulate it using futimes.
func (fs *loopbackFileSystem) Utimens(path string, a *time.Time, m *time.Time, context *fuse.Context) fuse.Status {
	tv := make([]syscall.Timeval, 2)
	if a == nil {
		tv[0].Usec = _UTIME_OMIT
	} else {
		tv[0] = timeToTimeval(a)
	}

	if m == nil {
		tv[1].Usec = _UTIME_OMIT
	} else {
		tv[1] = timeToTimeval(m)
	}

	err := syscall.Utimes(path, tv)
	return fuse.ToStatus(err)
}
