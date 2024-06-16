package dmesg // import "github.com/docker/docker/pkg/dmesg"

import (
	"golang.org/x/sys/unix"
)

// Dmesg returns last messages from the kernel log, up to size bytes.
//
// Deprecated: the dmesg package is no longer used, and will be removed in the next release.
func Dmesg(size int) []byte {
	b := make([]byte, size)
	amt, err := unix.Klogctl(unix.SYSLOG_ACTION_READ_ALL, b)
	if err != nil {
		return []byte{}
	}
	return b[:amt]
}
