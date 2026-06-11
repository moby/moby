//go:build !linux && !darwin && !freebsd && !netbsd

package fsutil

import "syscall"

var _ RootXattr = (*root)(nil)

func (r *root) LSetxattr(name, key string, value []byte, flags int) error {
	return unsupportedRootOp("lsetxattr", name, syscall.ENOSYS)
}
