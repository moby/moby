//go:build linux || darwin || freebsd || netbsd

package fsutil

import (
	"os"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

var _ RootXattr = (*root)(nil)

func (r *root) LSetxattr(name, key string, value []byte, flags int) error {
	f, err := r.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()

	if err := unix.Fsetxattr(int(f.Fd()), key, value, flags); err != nil {
		return errors.WithStack(&os.PathError{Op: "fsetxattr", Path: name, Err: err})
	}
	return nil
}
