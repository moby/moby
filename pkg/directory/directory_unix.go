//go:build linux || freebsd || darwin
// +build linux freebsd darwin

package directory // import "github.com/docker/docker/pkg/directory"

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
)

// Size walks a directory tree and returns its total size in bytes.
func Size(ctx context.Context, dir string) (size int64, err error) {
	data := make(map[uint64]struct{})
	err = filepath.Walk(dir, func(d string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			// if dir does not exist, Size() returns the error.
			// if dir/x disappeared while walking, Size() ignores dir/x.
			if os.IsNotExist(err) && d != dir {
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
		//nolint:unconvert // inode is not an uint64 on all platforms.
		if _, exists := data[uint64(inode)]; exists {
			return nil
		}

		data[uint64(inode)] = struct{}{} //nolint:unconvert // inode is not an uint64 on all platforms.

		size += s

		return nil
	})
	return
}
