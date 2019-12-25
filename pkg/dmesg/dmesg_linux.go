package dmesg // import "github.com/docker/docker/pkg/dmesg"

import (
	"golang.org/x/sys/unix"
)

// Dmesg returns last messages from the kernel log, up to size bytes
func Dmesg(size int) []byte {
	t := 3 // SYSLOG_ACTION_READ_ALL
	b := make([]byte, size)
	amt, err := unix.Klogctl(t, b)
	if err != nil {
		return []byte{}
	}
	return b[:amt]
}
