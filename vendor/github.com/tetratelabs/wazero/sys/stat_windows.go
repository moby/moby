//go:build (amd64 || arm64) && windows

package sys

import (
	"io/fs"
	"syscall"
)

const sysParseable = true

func statFromFileInfo(info fs.FileInfo) Stat_t {
	if d, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		st := Stat_t{}
		st.Ino = 0 // not in Win32FileAttributeData
		st.Dev = 0 // not in Win32FileAttributeData
		st.Mode = info.Mode()
		st.Nlink = 1 // not in Win32FileAttributeData
		st.Size = info.Size()
		st.Atim = d.LastAccessTime.Nanoseconds()
		st.Mtim = d.LastWriteTime.Nanoseconds()
		st.Ctim = d.CreationTime.Nanoseconds()
		return st
	}
	return defaultStatFromFileInfo(info)
}
