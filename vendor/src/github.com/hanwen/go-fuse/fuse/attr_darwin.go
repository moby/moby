package fuse

import (
	"syscall"
)

func (a *Attr) FromStat(s *syscall.Stat_t) {
	a.Ino = uint64(s.Ino)
	a.Size = uint64(s.Size)
	a.Blocks = uint64(s.Blocks)
	a.Atime = uint64(s.Atimespec.Sec)
	a.Atimensec = uint32(s.Atimespec.Nsec)
	a.Mtime = uint64(s.Mtimespec.Sec)
	a.Mtimensec = uint32(s.Mtimespec.Nsec)
	a.Ctime = uint64(s.Ctimespec.Sec)
	a.Ctimensec = uint32(s.Ctimespec.Nsec)
	a.Mode = uint32(s.Mode)
	a.Nlink = uint32(s.Nlink)
	a.Uid = uint32(s.Uid)
	a.Gid = uint32(s.Gid)
	a.Rdev = uint32(s.Rdev)
}
