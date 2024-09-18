package snapshot

import (
	"sync"

	"github.com/containerd/containerd/mount"
)

type Mounter interface {
	Mount() (string, error)
	Unmount() error
}

type LocalMounterOpt func(*localMounter)

// LocalMounter is a helper for mounting mountfactory to temporary path. In
// addition it can mount binds without privileges
func LocalMounter(mountable Mountable, opts ...LocalMounterOpt) Mounter {
	lm := &localMounter{mountable: mountable}
	for _, opt := range opts {
		opt(lm)
	}
	return lm
}

// LocalMounterWithMounts is a helper for mounting to temporary path. In
// addition it can mount binds without privileges
func LocalMounterWithMounts(mounts []mount.Mount, opts ...LocalMounterOpt) Mounter {
	lm := &localMounter{mounts: mounts}
	for _, opt := range opts {
		opt(lm)
	}
	return lm
}

type localMounter struct {
	mu           sync.Mutex
	mounts       []mount.Mount
	mountable    Mountable
	target       string
	release      func() error
	forceRemount bool
}

func ForceRemount() LocalMounterOpt {
	return func(lm *localMounter) {
		lm.forceRemount = true
	}
}
