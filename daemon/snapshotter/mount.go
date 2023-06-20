package snapshotter

import (
	"os"
	"path/filepath"

	"github.com/containerd/containerd/mount"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/locker"
	"github.com/sirupsen/logrus"
)

const mountsDir = "rootfs"

// List of known filesystems that can't be re-mounted or have shared layers
var refCountedFileSystems = []string{"fuse-overlayfs", "overlayfs", "stargz", "zfs"}

// Mounter handles mounting/unmounting things coming in from a snapshotter
// with optional reference counting if needed by the filesystem
type Mounter interface {
	// Mount mounts the rootfs for a container and returns the mount point
	Mount(mounts []mount.Mount, containerID string) (string, error)
	// Unmount unmounts the container rootfs
	Unmount(target string) error
}

// inSlice tests whether a string is contained in a slice of strings or not.
// Comparison is case sensitive
func inSlice(slice []string, s string) bool {
	for _, ss := range slice {
		if s == ss {
			return true
		}
	}
	return false
}

// NewMounter creates a new mounter for the provided snapshotter
func NewMounter(home string, snapshotter string, idMap idtools.IdentityMapping) Mounter {
	if inSlice(refCountedFileSystems, snapshotter) {
		return &refCountMounter{
			home:        home,
			snapshotter: snapshotter,
			rc:          graphdriver.NewRefCounter(checker()),
			locker:      locker.New(),
			idMap:       idMap,
		}
	}

	return mounter{
		home:        home,
		snapshotter: snapshotter,
		idMap:       idMap,
	}
}

type refCountMounter struct {
	home        string
	snapshotter string
	rc          *graphdriver.RefCounter
	locker      *locker.Locker
	idMap       idtools.IdentityMapping
}

func (m *refCountMounter) Mount(mounts []mount.Mount, containerID string) (target string, retErr error) {
	target = filepath.Join(m.home, mountsDir, m.snapshotter, containerID)

	_, err := os.Stat(target)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if count := m.rc.Increment(target); count > 1 {
		return target, nil
	}

	m.locker.Lock(target)
	defer m.locker.Unlock(target)

	defer func() {
		if retErr != nil {
			if c := m.rc.Decrement(target); c <= 0 {
				if mntErr := unmount(target); mntErr != nil {
					logrus.Errorf("error unmounting %s: %v", target, mntErr)
				}
				if rmErr := os.Remove(target); rmErr != nil && !os.IsNotExist(rmErr) {
					logrus.Debugf("Failed to remove %s: %v: %v", target, rmErr, err)
				}
			}
		}
	}()

	root := m.idMap.RootPair()
	if err := idtools.MkdirAllAndChown(target, 0700, root); err != nil {
		return "", err
	}

	return target, mount.All(mounts, target)
}

func (m *refCountMounter) Unmount(target string) error {
	if count := m.rc.Decrement(target); count > 0 {
		return nil
	}

	m.locker.Lock(target)
	defer m.locker.Unlock(target)

	if err := unmount(target); err != nil {
		logrus.Debugf("Failed to unmount %s: %v", target, err)
	}

	if err := os.Remove(target); err != nil {
		logrus.WithError(err).WithField("dir", target).Error("failed to remove mount temp dir")
	}

	return nil
}

type mounter struct {
	home        string
	snapshotter string
	idMap       idtools.IdentityMapping
}

func (m mounter) Mount(mounts []mount.Mount, containerID string) (string, error) {
	target := filepath.Join(m.home, mountsDir, m.snapshotter, containerID)

	root := m.idMap.RootPair()
	if err := idtools.MkdirAndChown(target, 0700, root); err != nil {
		return "", err
	}

	return target, mount.All(mounts, target)
}

func (m mounter) Unmount(target string) error {
	return unmount(target)

}
