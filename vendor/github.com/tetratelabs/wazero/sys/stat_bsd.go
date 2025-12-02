//go:build (amd64 || arm64) && (darwin || freebsd)

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
		st.Ino = d.Ino
		st.Mode = info.Mode()
		st.Nlink = uint64(d.Nlink)
		st.Size = d.Size
		atime := d.Atimespec
		st.Atim = atime.Sec*1e9 + atime.Nsec
		mtime := d.Mtimespec
		st.Mtim = mtime.Sec*1e9 + mtime.Nsec
		ctime := d.Ctimespec
		st.Ctim = ctime.Sec*1e9 + ctime.Nsec
		return st
	}
	return defaultStatFromFileInfo(info)
}
