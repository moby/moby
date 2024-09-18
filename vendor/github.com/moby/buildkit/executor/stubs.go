package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containerd/continuity/fs"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/system"
)

func MountStubsCleaner(ctx context.Context, dir string, mounts []Mount, recursive bool) func() {
	names := []string{"/etc/resolv.conf", "/etc/hosts"}

	for _, m := range mounts {
		names = append(names, m.Dest)
	}

	paths := make([]string, 0, len(names))

	for _, p := range names {
		p = filepath.Join("/", p)
		if p == "/" {
			continue
		}
		realPath, err := fs.RootPath(dir, p)
		if err != nil {
			continue
		}

		for {
			_, err = os.Lstat(realPath)
			if !(errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)) {
				break
			}
			paths = append(paths, realPath)

			if !recursive {
				break
			}

			realPathNext := filepath.Dir(realPath)
			if realPath == realPathNext || realPathNext == dir {
				break
			}
			realPath = realPathNext
		}
	}

	return func() {
		for _, p := range paths {
			p, err := fs.RootPath(dir, strings.TrimPrefix(p, dir))
			if err != nil {
				continue
			}

			st, err := os.Lstat(p)
			if err != nil {
				continue
			}
			if st.IsDir() {
				entries, err := os.ReadDir(p)
				if err != nil {
					continue
				}
				if len(entries) != 0 {
					continue
				}
			} else if st.Size() != 0 {
				continue
			}

			// Back up the timestamps of the dir for reproducible builds
			// https://github.com/moby/buildkit/issues/3148
			parent := filepath.Dir(p)
			if realPath, err := fs.RootPath(dir, strings.TrimPrefix(parent, dir)); err != nil || realPath != parent {
				continue
			}

			dirSt, err := os.Stat(parent)
			if err != nil {
				bklog.G(ctx).WithError(err).Warnf("Failed to stat %q (parent of mount stub %q)", dir, p)
				continue
			}
			mtime := dirSt.ModTime()
			atime, err := system.Atime(dirSt)
			if err != nil {
				bklog.G(ctx).WithError(err).Warnf("Failed to stat atime of %q (parent of mount stub %q)", dir, p)
				atime = mtime
			}

			if err := os.Remove(p); err != nil {
				bklog.G(ctx).WithError(err).Warnf("Failed to remove mount stub %q", p)
			}

			// Restore the timestamps of the dir
			if err := os.Chtimes(parent, atime, mtime); err != nil {
				bklog.G(ctx).WithError(err).Warnf("Failed to restore time time mount stub timestamp (os.Chtimes(%q, %v, %v))", dir, atime, mtime)
			}
		}
	}
}
