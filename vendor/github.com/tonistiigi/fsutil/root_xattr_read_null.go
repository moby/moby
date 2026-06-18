//go:build linux || darwin

package fsutil

import (
	"bytes"

	"golang.org/x/sys/unix"
)

func rootFlistxattr(fd int, buf []byte) (int, error) {
	return unix.Flistxattr(fd, buf)
}

func rootParseListxattr(buf []byte) []string {
	parts := bytes.Split(bytes.TrimSuffix(buf, []byte{0}), []byte{0})
	var xattrs []string
	for _, part := range parts {
		if len(part) > 0 {
			xattrs = append(xattrs, string(part))
		}
	}
	return xattrs
}
