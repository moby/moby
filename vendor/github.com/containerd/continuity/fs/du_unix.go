// +build !windows

package fs

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
)

type inode struct {
	// TODO(stevvooe): Can probably reduce memory usage by not tracking
	// device, but we can leave this right for now.
	dev, ino uint64
}

func newInode(stat *syscall.Stat_t) inode {
	return inode{
		// Dev is uint32 on darwin/bsd, uint64 on linux/solaris
		dev: uint64(stat.Dev), // nolint: unconvert
		// Ino is uint32 on bsd, uint64 on darwin/linux/solaris
		ino: uint64(stat.Ino), // nolint: unconvert
	}
}

func diskUsage(roots ...string) (Usage, error) {

	var (
		size   int64
		inodes = map[inode]struct{}{} // expensive!
	)

	for _, root := range roots {
		if err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			inoKey := newInode(fi.Sys().(*syscall.Stat_t))
			if _, ok := inodes[inoKey]; !ok {
				inodes[inoKey] = struct{}{}
				size += fi.Size()
			}

			return nil
		}); err != nil {
			return Usage{}, err
		}
	}

	return Usage{
		Inodes: int64(len(inodes)),
		Size:   size,
	}, nil
}

func diffUsage(ctx context.Context, a, b string) (Usage, error) {
	var (
		size   int64
		inodes = map[inode]struct{}{} // expensive!
	)

	if err := Changes(ctx, a, b, func(kind ChangeKind, _ string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if kind == ChangeKindAdd || kind == ChangeKindModify {
			inoKey := newInode(fi.Sys().(*syscall.Stat_t))
			if _, ok := inodes[inoKey]; !ok {
				inodes[inoKey] = struct{}{}
				size += fi.Size()
			}

			return nil

		}
		return nil
	}); err != nil {
		return Usage{}, err
	}

	return Usage{
		Inodes: int64(len(inodes)),
		Size:   size,
	}, nil
}
