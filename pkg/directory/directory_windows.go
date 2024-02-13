package directory // import "github.com/docker/docker/pkg/directory"

import (
	"context"
	"os"
	"path/filepath"
)

// calcSize walks a directory tree and returns its total calcSize in bytes.
func calcSize(ctx context.Context, dir string) (int64, error) {
	var size int64
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

		size += s

		return nil
	})
	return size, err
}
