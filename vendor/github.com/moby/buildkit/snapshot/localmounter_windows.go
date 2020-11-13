package snapshot

import (
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/pkg/errors"
)

func (lm *localMounter) Mount() (string, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.mounts == nil {
		mounts, release, err := lm.mountable.Mount()
		if err != nil {
			return "", err
		}
		lm.mounts = mounts
		lm.release = release
	}

	// Windows can only mount a single mount at a given location.
	// Parent layers are carried in Options, opaquely to localMounter.
	if len(lm.mounts) != 1 {
		return "", errors.Wrapf(errdefs.ErrNotImplemented, "request to mount %d layers, only 1 is supported", len(lm.mounts))
	}

	m := lm.mounts[0]

	if m.Type == "bind" || m.Type == "rbind" {
		ro := false
		for _, opt := range m.Options {
			if opt == "ro" {
				ro = true
				break
			}
		}
		if !ro {
			return m.Source, nil
		}
	}

	// Windows mounts always activate in-place, so the target of the mount must be the source directory.
	// See https://github.com/containerd/containerd/pull/2366
	dir := m.Source

	if err := m.Mount(dir); err != nil {
		return "", errors.Wrapf(err, "failed to mount in-place: %v", m)
	}
	lm.target = dir
	return lm.target, nil
}

func (lm *localMounter) Unmount() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.target != "" {
		if err := mount.Unmount(lm.target, 0); err != nil {
			return err
		}
		lm.target = ""
	}

	if lm.release != nil {
		return lm.release()
	}

	return nil
}
