//go:build linux || freebsd || darwin

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
		//nolint:unconvert // inode is not an uint64 on all platforms.
		if _, exists := data[uint64(inode)]; exists {
			return nil
		}

		data[uint64(inode)] = struct{}{} //nolint:unconvert // inode is not an uint64 on all platforms.

		size += s

		return nil
	})
	return size, err
}
