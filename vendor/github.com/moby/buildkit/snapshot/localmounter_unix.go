//go:build !windows
// +build !windows

package snapshot

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/pkg/userns"
	rootlessmountopts "github.com/moby/buildkit/util/rootless/mountopts"
	"github.com/pkg/errors"
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

	if userns.RunningInUserNS() {
		var err error
		lm.mounts, err = rootlessmountopts.FixUp(lm.mounts)
		if err != nil {
			return "", err
		}
	}

	var isFile bool
	if len(lm.mounts) == 1 && (lm.mounts[0].Type == "bind" || lm.mounts[0].Type == "rbind") {
		if !lm.forceRemount {
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
		fi, err := os.Stat(lm.mounts[0].Source)
		if err != nil {
			return "", err
		}
		if !fi.IsDir() {
			isFile = true
		}
	}

	dest, err := os.MkdirTemp("", "buildkit-mount")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp dir")
	}

	if isFile {
		dest = filepath.Join(dest, "file")
		if err := os.WriteFile(dest, []byte{}, 0644); err != nil {
			os.RemoveAll(dest)
			return "", errors.Wrap(err, "failed to create temp file")
		}
	}

	if err := mount.All(lm.mounts, dest); err != nil {
		os.RemoveAll(dest)
		return "", errors.Wrapf(err, "failed to mount %s: %+v", dest, lm.mounts)
	}
	lm.target = dest
	return dest, nil
}

func (lm *localMounter) Unmount() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.target != "" {
		if err := mount.Unmount(lm.target, syscall.MNT_DETACH); err != nil {
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
