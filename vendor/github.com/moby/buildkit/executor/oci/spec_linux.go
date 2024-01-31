//go:build linux
// +build linux

package oci

import (
	"os"
	"strconv"

	"github.com/containerd/containerd/mount"
	"github.com/containerd/continuity/fs"
	"github.com/moby/buildkit/snapshot"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func sub(m mount.Mount, subPath string) (mount.Mount, func() error, error) {
	var retries = 10
	root := m.Source
	for {
		src, err := fs.RootPath(root, subPath)
		if err != nil {
			return mount.Mount{}, nil, err
		}
		// similar to runc.WithProcfd
		fh, err := os.OpenFile(src, unix.O_PATH|unix.O_CLOEXEC, 0)
		if err != nil {
			return mount.Mount{}, nil, err
		}

		fdPath := "/proc/self/fd/" + strconv.Itoa(int(fh.Fd()))
		if resolved, err := os.Readlink(fdPath); err != nil {
			fh.Close()
			return mount.Mount{}, nil, err
		} else if resolved != src {
			retries--
			if retries <= 0 {
				fh.Close()
				return mount.Mount{}, nil, errors.Errorf("unable to safely resolve subpath %s", subPath)
			}
			fh.Close()
			continue
		}

		m.Source = fdPath
		lm := snapshot.LocalMounterWithMounts([]mount.Mount{m}, snapshot.ForceRemount())
		mp, err := lm.Mount()
		if err != nil {
			fh.Close()
			return mount.Mount{}, nil, err
		}
		m.Source = mp
		fh.Close() // release the fd, we don't need it anymore

		return m, lm.Unmount, nil
	}
}
