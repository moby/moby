package fs

import (
	"syscall"
	"time"
)

// StatAtime returns the Atim
func StatAtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Atim
}

// StatCtime returns the Ctim
func StatCtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Ctim
}

// StatMtime returns the Mtim
func StatMtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Mtim
}

// StatATimeAsTime returns st.Atim as a time.Time
func StatATimeAsTime(st *syscall.Stat_t) time.Time {
	// The int64 conversions ensure the line compiles for 32-bit systems as well.
	return time.Unix(int64(st.Atim.Sec), int64(st.Atim.Nsec)) // nolint: unconvert
}
