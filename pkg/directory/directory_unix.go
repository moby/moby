//go:build linux || freebsd || darwin
// +build linux freebsd darwin

package directory // import "github.com/docker/docker/pkg/directory"

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
)

// calcSize walks a directory tree and returns its total size in bytes.
func calcSize(ctx context.Context, dir string) (int64, error) {
	var size int64
	data := make(map[uint64]struct{})
	err := filepath.Walk(dir, func(d string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			// if dir/x disappeared while walking, Size() ignores dir/x.
			// if dir does not exist, Size() returns the error.
			if d != dir && os.IsNotExist(err) {
				return nil
			}
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

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
		// inode is not a uint64 on all platforms. Cast it to avoid issues.
		if _, exists := data[inode]; exists {
			return nil
		}
		// inode is not a uint64 on all platforms. Cast it to avoid issues.
		data[inode] = struct{}{}

		size += s

		return nil
	})
	return size, err
}
