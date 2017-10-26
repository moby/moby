// +build linux solaris

package sys

import (
	"syscall"
)

func StatAtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Atim
}

func StatCtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Ctim
}

func StatMtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Mtim
}
