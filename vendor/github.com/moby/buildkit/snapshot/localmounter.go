package snapshot

import (
	"sync"

	"github.com/containerd/containerd/mount"
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
	release   func() error
}
