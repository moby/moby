// +build darwin freebsd

package sys

import (
	"syscall"
)

func StatAtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Atimespec
}

func StatCtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Ctimespec
}

func StatMtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Mtimespec
}
