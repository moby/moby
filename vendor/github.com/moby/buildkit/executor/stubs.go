package executor

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containerd/continuity/fs"
)

func MountStubsCleaner(dir string, mounts []Mount) func() {
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

		_, err = os.Lstat(realPath)
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			paths = append(paths, realPath)
		}
	}

	return func() {
		for _, p := range paths {
			st, err := os.Lstat(p)
			if err != nil {
				continue
			}
			if st.Size() != 0 {
				continue
			}
			os.Remove(p)
		}
	}
}
