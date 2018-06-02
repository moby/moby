// +build !linux

package fsutil

import (
	"os"
	"time"
)

func chtimes(path string, un int64) error {
	mtime := time.Unix(0, un)
	return os.Chtimes(path, mtime, mtime)
}
