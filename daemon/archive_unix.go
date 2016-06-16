// +build !windows

package daemon

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/container"
)

// checkIfPathIsInAVolume checks if the path is in a volume. If it is, it
// cannot be in a read-only volume. If it  is not in a volume, the container
// cannot be configured with a read-only rootfs.
func checkIfPathIsInAVolume(container *container.Container, absPath string) (bool, error) {
	var toVolume bool
	for _, mnt := range container.MountPoints {
		if toVolume = mnt.HasResource(absPath); toVolume {
			if mnt.RW {
				break
			}
			return false, ErrVolumeReadonly
		}
	}
	return toVolume, nil
}

func fixPermissions(source, destination string, uid, gid int, destExisted bool) error {
	// If the destination didn't already exist, or the destination isn't a
	// directory, then we should Lchown the destination. Otherwise, we shouldn't
	// Lchown the destination.
	destStat, err := os.Stat(destination)
	if err != nil {
		// This should *never* be reached, because the destination must've already
		// been created while untar-ing the context.
		return err
	}
	doChownDestination := !destExisted || !destStat.IsDir()

	// We Walk on the source rather than on the destination because we don't
	// want to change permissions on things we haven't created or modified.
	return filepath.Walk(source, func(fullpath string, info os.FileInfo, err error) error {
		// Do not alter the walk root iff. it existed before, as it doesn't fall under
		// the domain of "things we should chown".
		if !doChownDestination && (source == fullpath) {
			return nil
		}

		// Path is prefixed by source: substitute with destination instead.
		cleaned, err := filepath.Rel(source, fullpath)
		if err != nil {
			return err
		}

		fullpath = filepath.Join(destination, cleaned)
		return os.Lchown(fullpath, uid, gid)
	})
}
