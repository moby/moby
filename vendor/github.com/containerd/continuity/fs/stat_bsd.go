// +build darwin freebsd

package fs

import (
	"syscall"
	"time"
)

// StatAtime returns the access time from a stat struct
func StatAtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Atimespec
}

// StatCtime returns the created time from a stat struct
func StatCtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Ctimespec
}

// StatMtime returns the modified time from a stat struct
func StatMtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Mtimespec
}

// StatATimeAsTime returns the access time as a time.Time
func StatATimeAsTime(st *syscall.Stat_t) time.Time {
	return time.Unix(int64(st.Atimespec.Sec), int64(st.Atimespec.Nsec)) // nolint: unconvert
}
