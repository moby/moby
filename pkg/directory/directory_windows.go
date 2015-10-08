// +build windows

package directory

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/longpath"
)

// Size walks a directory tree and returns its total size in bytes.
func Size(dir string) (size int64, err error) {
	fixedPath, err := filepath.Abs(dir)
	if err != nil {
		return
	}
	fixedPath = longpath.AddPrefix(fixedPath)
	err = filepath.Walk(dir, func(d string, fileInfo os.FileInfo, e error) error {
		// Ignore directory sizes
		if fileInfo == nil {
			return nil
		}

		s := fileInfo.Size()
		if fileInfo.IsDir() || s == 0 {
			return nil
		}

		size += s

		return nil
	})
	return
}
