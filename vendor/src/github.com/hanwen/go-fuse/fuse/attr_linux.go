package fuse

import (
	"syscall"
)

func (a *Attr) FromStat(s *syscall.Stat_t) {
	a.Ino = uint64(s.Ino)
	a.Size = uint64(s.Size)
	a.Blocks = uint64(s.Blocks)
	a.Atime = uint64(s.Atim.Sec)
	a.Atimensec = uint32(s.Atim.Nsec)
	a.Mtime = uint64(s.Mtim.Sec)
	a.Mtimensec = uint32(s.Mtim.Nsec)
	a.Ctime = uint64(s.Ctim.Sec)
	a.Ctimensec = uint32(s.Ctim.Nsec)
	a.Mode = s.Mode
	a.Nlink = uint32(s.Nlink)
	a.Uid = uint32(s.Uid)
	a.Gid = uint32(s.Gid)
	a.Rdev = uint32(s.Rdev)
	a.Blksize = uint32(s.Blksize)
}
