package snapshot

import (
	"io/ioutil"
	"os"
	"sync"

	"github.com/containerd/containerd/mount"
	"github.com/pkg/errors"
)

type Mounter interface {
	Mount() (string, error)
	Unmount() error
}

// LocalMounter is a helper for mounting mountfactory to temporary path. In
// addition it can mount binds without privileges
func LocalMounter(mountable Mountable) Mounter {
	return &localMounter{mountable: mountable}
}

// LocalMounterWithMounts is a helper for mounting to temporary path. In
// addition it can mount binds without privileges
func LocalMounterWithMounts(mounts []mount.Mount) Mounter {
	return &localMounter{mounts: mounts}
}

type localMounter struct {
	mu        sync.Mutex
	mounts    []mount.Mount
	mountable Mountable
	target    string
}

func (lm *localMounter) Mount() (string, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.mounts == nil {
		mounts, err := lm.mountable.Mount()
		if err != nil {
			return "", err
		}
		lm.mounts = mounts
	}

	if len(lm.mounts) == 1 && (lm.mounts[0].Type == "bind" || lm.mounts[0].Type == "rbind") {
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

	dir, err := ioutil.TempDir("", "buildkit-mount")
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
