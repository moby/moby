//go:build (linux || darwin || freebsd || netbsd || dragonfly || solaris) && !tinygo

package platform

import "syscall"

func munmapCodeSegment(code []byte) error {
	return syscall.Munmap(code)
}
