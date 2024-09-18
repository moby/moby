package overlay

import "github.com/containerd/containerd/mount"

// IsOverlayMountType returns true if the mount type is overlay-based
func IsOverlayMountType(mnt mount.Mount) bool {
	return mnt.Type == "overlay"
}
