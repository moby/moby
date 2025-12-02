//go:build (amd64 || arm64 || riscv64) && linux

// Note: This expression is not the same as compiler support, even if it looks
// similar. Platform functions here are used in interpreter mode as well.

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
		st.Ino = uint64(d.Ino)
		st.Mode = info.Mode()
		st.Nlink = uint64(d.Nlink)
		st.Size = d.Size
		atime := d.Atim
		st.Atim = atime.Sec*1e9 + atime.Nsec
		mtime := d.Mtim
		st.Mtim = mtime.Sec*1e9 + mtime.Nsec
		ctime := d.Ctim
		st.Ctim = ctime.Sec*1e9 + ctime.Nsec
		return st
	}
	return defaultStatFromFileInfo(info)
}
