//go:build !windows

/*
   Copyright The Accelerated Container Image Authors

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Based on https://github.com/containerd/continuity/blob/main/fs/du_unix.go
// Used to calculate the usage of the block dir, excluding the block/mountpoint dir.

package utils

import (
	"context"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containerd/continuity/fs"
)

const blocksUnitSize = 512

type inode struct {
	dev, ino uint64
}

func newInode(stat *syscall.Stat_t) inode {
	return inode{
		dev: uint64(stat.Dev), //nolint: unconvert // dev is uint32 on darwin/bsd, uint64 on linux/solaris/freebsd
		ino: uint64(stat.Ino), //nolint: unconvert // ino is uint32 on bsd, uint64 on darwin/linux/solaris/freebsd
	}
}

func DiskUsageWithoutMountpoint(ctx context.Context, roots ...string) (fs.Usage, error) {
	var (
		size   int64
		inodes = map[inode]struct{}{} // expensive!
	)

	for _, root := range roots {
		if err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if fi.Name() == "mountpoint" {
				return filepath.SkipDir
			}
			if err != nil {
				return err
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			stat := fi.Sys().(*syscall.Stat_t)
			inoKey := newInode(stat)
			if _, ok := inodes[inoKey]; !ok {
				inodes[inoKey] = struct{}{}
				size += stat.Blocks * blocksUnitSize
			}

			return nil
		}); err != nil {
			return fs.Usage{}, err
		}
	}

	return fs.Usage{
		Inodes: int64(len(inodes)),
		Size:   size,
	}, nil
}
