//go:build linux || openbsd || dragonfly || solaris

package sys

import (
	"io/fs"
	"syscall"
)

const sysParseable = true

func statFromFileInfo(info fs.FileInfo) Stat_t {
	if d, ok := info.Sys().(*syscall.Stat_t); ok {
		st := Stat_t{}
		st.Dev = uint64(d.Dev)
		st.Ino = Inode(d.Ino)
		st.Mode = info.Mode()
		st.Nlink = uint64(d.Nlink)
		st.Size = int64(d.Size)
		atime := d.Atim
		st.Atim = EpochNanos(atime.Sec)*1e9 + EpochNanos(atime.Nsec)
		mtime := d.Mtim
		st.Mtim = EpochNanos(mtime.Sec)*1e9 + EpochNanos(mtime.Nsec)
		ctime := d.Ctim
		st.Ctim = EpochNanos(ctime.Sec)*1e9 + EpochNanos(ctime.Nsec)
		return st
	}
	return defaultStatFromFileInfo(info)
}
