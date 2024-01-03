package snapshot

import (
	"os"
	"sync/atomic"

	"github.com/containerd/containerd/mount"
	"github.com/docker/docker/pkg/idtools"
)

type staticMountable struct {
	count  int32
	id     string
	mounts []mount.Mount
	idmap  *idtools.IdentityMapping
}

func (cm *staticMountable) Mount() ([]mount.Mount, func() error, error) {
	// return a copy to prevent changes to mount.Mounts in the slice from affecting cm
	mounts := make([]mount.Mount, len(cm.mounts))
	copy(mounts, cm.mounts)

	redirectDirOption := getRedirectDirOption()
	if redirectDirOption != "" {
		mounts = setRedirectDir(mounts, redirectDirOption)
	}

	atomic.AddInt32(&cm.count, 1)
	return mounts, func() error {
		if atomic.AddInt32(&cm.count, -1) < 0 {
			if v := os.Getenv("BUILDKIT_DEBUG_PANIC_ON_ERROR"); v == "1" {
				panic("release of released mount " + cm.id)
			}
		}
		return nil
	}, nil
}

func (cm *staticMountable) IdentityMapping() *idtools.IdentityMapping {
	return cm.idmap
}
