//go:build windows

package fsutil

import (
	"os"
	"time"

	"github.com/pkg/errors"
)

var _ RootLChtimes = (*root)(nil)

func (r *root) LChtimes(name string, mtime time.Time) error {
	fi, err := r.Lstat(name)
	if err != nil {
		return errors.WithStack(err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if err := r.Chtimes(name, mtime, mtime); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
