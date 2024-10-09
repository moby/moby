package snapshot

import (
	"os"

	"github.com/containerd/containerd/mount"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func (lm *localMounter) Mount() (string, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.mounts == nil && lm.mountable != nil {
		mounts, release, err := lm.mountable.Mount()
		if err != nil {
			return "", err
		}
		lm.mounts = mounts
		lm.release = release
	}

	if len(lm.mounts) == 1 && lm.mounts[0].Type == "bind" {
		ro := false
		for _, opt := range lm.mounts[0].Options {
			if opt == "ro" {
				ro = true
				break
			}
		}
		if !ro {
			return lm.mounts[0].Source, nil
		}
	}

	dir, err := os.MkdirTemp("", "buildkit-mount")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp dir")
	}

	if err := mount.All(lm.mounts, dir); err != nil {
		os.RemoveAll(dir)
		return "", errors.Wrapf(err, "failed to mount %s: %+v", dir, lm.mounts)
	}
	lm.target = dir
	return dir, nil
}

func (lm *localMounter) Unmount() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.target != "" {
		if err := mount.UnmountRecursive(lm.target, unix.MNT_FORCE); err != nil {
			return err
		}
		os.RemoveAll(lm.target)
		lm.target = ""
	}

	if lm.release != nil {
		return lm.release()
	}

	return nil
}
