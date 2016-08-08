package nodefs

import (
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

func (f *loopbackFile) Allocate(off uint64, sz uint64, mode uint32) fuse.Status {
	f.lock.Lock()
	err := syscall.Fallocate(int(f.File.Fd()), mode, int64(off), int64(sz))
	f.lock.Unlock()
	if err != nil {
		return fuse.ToStatus(err)
	}
	return fuse.OK
}

const _UTIME_NOW = ((1 << 30) - 1)
const _UTIME_OMIT = ((1 << 30) - 2)

// Utimens - file handle based version of loopbackFileSystem.Utimens()
func (f *loopbackFile) Utimens(a *time.Time, m *time.Time) fuse.Status {
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
		ts[1] = syscall.NsecToTimespec(a.UnixNano())
		ts[1].Nsec = 0
	}

	f.lock.Lock()
	err := futimens(int(f.File.Fd()), &ts)
	f.lock.Unlock()
	return fuse.ToStatus(err)
}
