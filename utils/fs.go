package utils

import (
	"os"
	"path/filepath"
	"syscall"
)

// TreeSize walks a directory tree and returns its total size in bytes.
func TreeSize(dir string) (size int64, err error) {
	data := make(map[uint64]bool)
	err = filepath.Walk(dir, func(d string, fileInfo os.FileInfo, e error) error {
		// Ignore directory sizes
		if fileInfo == nil {
			return nil
		}

		s := fileInfo.Size()
		if fileInfo.IsDir() || s == 0 {
			return nil
		}

		// Check inode to handle hard links correctly
		inode := fileInfo.Sys().(*syscall.Stat_t).Ino
		if _, exists := data[inode]; exists {
			return nil
		}
		data[inode] = false

		size += s

		return nil
	})
	return
}
