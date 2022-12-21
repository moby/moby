package executor

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containerd/continuity/fs"
	"github.com/moby/buildkit/util/system"
	"github.com/sirupsen/logrus"
)

func MountStubsCleaner(dir string, mounts []Mount, recursive bool) func() {
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
			if realPath == realPathNext {
				break
			}
			realPath = realPathNext
		}
	}

	return func() {
		for _, p := range paths {
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
			dir := filepath.Dir(p)
			dirSt, err := os.Stat(dir)
			if err != nil {
				logrus.WithError(err).Warnf("Failed to stat %q (parent of mount stub %q)", dir, p)
				continue
			}
			mtime := dirSt.ModTime()
			atime, err := system.Atime(dirSt)
			if err != nil {
				logrus.WithError(err).Warnf("Failed to stat atime of %q (parent of mount stub %q)", dir, p)
				atime = mtime
			}

			if err := os.Remove(p); err != nil {
				logrus.WithError(err).Warnf("Failed to remove mount stub %q", p)
			}

			// Restore the timestamps of the dir
			if err := os.Chtimes(dir, atime, mtime); err != nil {
				logrus.WithError(err).Warnf("Failed to restore time time mount stub timestamp (os.Chtimes(%q, %v, %v))", dir, atime, mtime)
			}
		}
	}
}
