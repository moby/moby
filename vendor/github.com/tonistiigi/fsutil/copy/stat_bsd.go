// +build darwin freebsd netbsd openbsd

package fs

import (
	"syscall"
)

// Returns the last-accessed time
func StatAtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Atimespec
}

// Returns the last-modified time
func StatMtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Mtimespec
}
