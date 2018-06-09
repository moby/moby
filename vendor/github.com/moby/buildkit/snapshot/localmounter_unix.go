// +build !windows

package snapshot

import (
	"os"
	"syscall"

	"github.com/containerd/containerd/mount"
)

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

	if lm.mountable != nil {
		return lm.mountable.Release()
	}

	return nil
}
