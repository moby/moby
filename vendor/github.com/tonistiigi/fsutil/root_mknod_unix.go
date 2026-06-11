//go:build linux || netbsd || openbsd || dragonfly

package fsutil

import (
	"os"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

var _ RootMknod = (*root)(nil)

func (r *root) Mknod(name string, mode uint32, dev int) error {
	parent, base, closeParent, err := r.openRootParent(name)
	if err != nil {
		return err
	}
	if closeParent {
		defer parent.Close()
	}

	if err := unix.Mknodat(int(parent.Fd()), base, mode, dev); err != nil {
		return errors.WithStack(&os.PathError{Op: "mknodat", Path: name, Err: err})
	}
	return nil
}
