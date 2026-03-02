package sysfs

import (
	"syscall"

	"github.com/tetratelabs/wazero/experimental/sys"
)

func utimens(path string, atim, mtim int64) sys.Errno {
	return chtimes(path, atim, mtim)
}

func futimens(fd uintptr, atim, mtim int64) error {
	// Per docs, zero isn't a valid timestamp as it cannot be differentiated
	// from nil. In both cases, it is a marker like sys.UTIME_OMIT.
	// See https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-setfiletime
	a, w := timespecToFiletime(atim, mtim)

	if a == nil && w == nil {
		return nil // both omitted, so nothing to change
	}

	// Attempt to get the stat by handle, which works for normal files
	h := syscall.Handle(fd)

	// Note: This returns ERROR_ACCESS_DENIED when the input is a directory.
	return syscall.SetFileTime(h, nil, a, w)
}

func timespecToFiletime(atim, mtim int64) (a, w *syscall.Filetime) {
	a = timespecToFileTime(atim)
	w = timespecToFileTime(mtim)
	return
}

func timespecToFileTime(tim int64) *syscall.Filetime {
	if tim == sys.UTIME_OMIT {
		return nil
	}
	ft := syscall.NsecToFiletime(tim)
	return &ft
}
