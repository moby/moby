// +build !linux

package fsutil

import (
	"os"
	"time"
)

func chtimes(path string, un int64) error {
	mtime := time.Unix(0, un)
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	return os.Chtimes(path, mtime, mtime)
}
