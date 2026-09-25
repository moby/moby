//go:build !linux && !freebsd && !netbsd && !openbsd && !dragonfly

package fsutil

import "syscall"

var _ RootMknod = (*root)(nil)

func (r *root) Mknod(name string, mode uint32, dev int) error {
	return unsupportedRootOp("mknodat", name, syscall.ENOSYS)
}
