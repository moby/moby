//go:build !linux
// +build !linux

package fsutil

import (
	"os"
	"time"

	"github.com/pkg/errors"
)

func chtimes(path string, un int64) error {
	mtime := time.Unix(0, un)
	fi, err := os.Lstat(path)
	if err != nil {
		return errors.WithStack(err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	return errors.WithStack(os.Chtimes(path, mtime, mtime))
}
