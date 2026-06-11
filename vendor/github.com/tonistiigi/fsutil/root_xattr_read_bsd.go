//go:build freebsd || netbsd

package fsutil

import "golang.org/x/sys/unix"

func rootFlistxattr(fd int, buf []byte) (int, error) {
	return unix.FlistxattrNS(fd, unix.EXTATTR_NAMESPACE_USER, buf)
}

func rootParseListxattr(buf []byte) []string {
	var xattrs []string
	for i := 0; i < len(buf); {
		next := i + 1 + int(buf[i])
		if next > len(buf) {
			break
		}
		if next > i+1 {
			xattrs = append(xattrs, "user."+string(buf[i+1:next]))
		}
		i = next
	}
	return xattrs
}
