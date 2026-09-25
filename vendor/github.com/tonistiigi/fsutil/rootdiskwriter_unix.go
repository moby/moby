//go:build !windows

package fsutil

import (
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil/types"
)

func rewriteRootMetadata(root Root, p string, stat *types.Stat) error {
	for key, value := range stat.Xattrs {
		root.LSetxattr(p, key, value, 0)
	}

	if err := root.Lchown(p, int(stat.Uid), int(stat.Gid)); err != nil {
		return errors.WithStack(err)
	}

	if os.FileMode(stat.Mode)&os.ModeSymlink != 0 {
		return root.LChtimes(p, time.Unix(0, stat.ModTime))
	}
	if err := root.Chmod(p, os.FileMode(stat.Mode)); err != nil {
		return errors.WithStack(err)
	}
	if err := rootChtimes(root, p, stat.ModTime); err != nil {
		return err
	}

	return nil
}
