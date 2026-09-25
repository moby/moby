//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly && !windows

package fsutil

import (
	"syscall"
	"time"
)

var _ RootLChtimes = (*root)(nil)

func (r *root) LChtimes(name string, mtime time.Time) error {
	return unsupportedRootOp("utimensat", name, syscall.ENOSYS)
}
