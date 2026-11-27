package snapshotter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/core/mount"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/moby/locker"
	"github.com/moby/moby/v2/daemon/internal/mountref"
	"github.com/moby/moby/v2/daemon/internal/stringid"
	"github.com/moby/sys/mountinfo"
	"github.com/moby/sys/user"
)

// Mounter handles mounting/unmounting things coming in from a snapshotter
// with optional reference counting if needed by the filesystem
type Mounter interface {
	// Mount mounts the rootfs for a container and returns the mount point
	Mount(mounts []mount.Mount, containerID string) (string, error)
	// Unmount unmounts the container rootfs
	Unmount(target string) error
	// Mounted returns a target mountpoint if it's already mounted
	Mounted(containerID string) (string, error)
	// WithTempMount mounts the mounts to a temporary directory for the duration of f
	WithTempMount(ctx context.Context, mounts []mount.Mount, f func(root string) error) error
	// WithReadonlyTempMount is WithTempMount with the mounts made read-only
	WithReadonlyTempMount(ctx context.Context, mounts []mount.Mount, f func(root string) error) error
}

// NewMounter creates a new mounter for the provided snapshotter
func NewMounter(home string, snapshotter string, idMap user.IdentityMapping, client *containerd.Client) *refCountMounter {
	return &refCountMounter{
		mm:     client.MountManager(),
		leases: client.LeasesService(),
		base: mounter{
			home:        home,
			snapshotter: snapshotter,
			idMap:       idMap,
		},
		rc:     mountref.NewCounter(isMounted),
		locker: locker.New(),
	}
}

type refCountMounter struct {
	rc     *mountref.Counter
	locker *locker.Locker
	base   mounter
	mm     mount.Manager
	leases leases.Manager
}

func (m *refCountMounter) Mount(mounts []mount.Mount, containerID string) (target string, retErr error) {
	target = m.base.target(containerID)

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
					log.G(context.TODO()).Errorf("error unmounting %s: %v", target, mntErr)
				}

				if err := m.deactivateTarget(context.TODO(), target); err != nil {
					log.G(context.TODO()).WithFields(log.Fields{
						"error":  err,
						"target": target,
					}).Error("failed to clean up after failed mount")
				}

				if rmErr := os.Remove(target); rmErr != nil && !os.IsNotExist(rmErr) {
					log.G(context.TODO()).Debugf("Failed to remove %s: %v: %v", target, rmErr, err)
				}
			}
		}
	}()

	// The label ties the activation to the container's snapshot, so containerd GC
	// keeps it while the snapshot exists and can collect it once the snapshot is removed.
	activeMounts, err := m.activate(context.TODO(), m.activationKey(target), mounts, mount.WithLabels(map[string]string{
		"containerd.io/gc.bref.snapshot." + m.base.snapshotter: containerID,
	}))
	if err == nil {
		mounts = activeMounts
	} else if !cerrdefs.IsNotImplemented(err) {
		return target, err
	}

	// Return target on failure too, the deferred cleanup needs it to drop the reference.
	_, err = m.base.Mount(mounts, containerID)
	return target, err
}

func (m *refCountMounter) WithTempMount(ctx context.Context, mounts []mount.Mount, f func(root string) error) error {
	return m.withTempMount(ctx, mounts, false, f)
}

func (m *refCountMounter) WithReadonlyTempMount(ctx context.Context, mounts []mount.Mount, f func(root string) error) error {
	return m.withTempMount(ctx, mounts, true, f)
}

// withTempMount keeps the mounts activated while they are mounted temporarily.
func (m *refCountMounter) withTempMount(ctx context.Context, mounts []mount.Mount, readonly bool, f func(root string) error) error {
	if m.skipMountManager() {
		return tempMount(ctx, mounts, readonly, f)
	}

	key := "moby-" + m.base.snapshotter + "-tmp-" + stringid.GenerateRandomID()

	// Activate attaches the activation to the lease in ctx, and a caller's lease
	// may never expire. Use a dedicated expiring lease so GC can reclaim the
	// activation if dockerd exits before Deactivate.
	lease, err := m.leases.Create(ctx, leases.WithID(key), leases.WithExpiration(8*time.Hour))
	if err != nil {
		return fmt.Errorf("failed to create temporary mount lease: %w", err)
	}
	defer func() {
		if err := m.leases.Delete(context.WithoutCancel(ctx), lease); err != nil && !cerrdefs.IsNotFound(err) {
			log.G(ctx).WithFields(log.Fields{
				"error": err,
				"lease": lease.ID,
			}).Warn("failed to release temporary mount lease")
		}
	}()

	activeMounts, err := m.activate(leases.WithLease(ctx, lease.ID), key, mounts)
	if !cerrdefs.IsNotImplemented(err) {
		// A failed Activate may still have recorded the activation.
		defer func() {
			// f has returned, so a failed deactivation is only logged and the lease
			// lets GC reclaim what is left. containerd's Deactivate can fail with
			// ENOENT when GC unmounts the activation after Deactivate deleted its record.
			if err := m.deactivate(context.WithoutCancel(ctx), key); err != nil {
				log.G(ctx).WithFields(log.Fields{
					"error": err,
					"key":   key,
				}).Warn("failed to deactivate temporary mounts")
			}
		}()

		if err != nil {
			return err
		}

		mounts = activeMounts
	}

	return tempMount(ctx, mounts, readonly, f)
}

func tempMount(ctx context.Context, mounts []mount.Mount, readonly bool, f func(root string) error) error {
	if readonly {
		return mount.WithReadonlyTempMount(ctx, mounts, f)
	}
	return mount.WithTempMount(ctx, mounts, f)
}

// skipMountManager reports whether the mounts of the snapshotter bypass containerd's
// Mount Manager. The listed snapshotters return only mounts that dockerd performs
// itself, so there is nothing to activate for them.
//
// A hard-coded list is not ideal, as a listed snapshotter could start returning mounts
// that need activation. See https://github.com/moby/moby/pull/54005#discussion_r4720407992
func (m *refCountMounter) skipMountManager() bool {
	switch m.base.snapshotter {
	case "overlayfs", "fuse-overlayfs", "native":
		return true
	default:
		return false
	}
}

// activate activates the mounts with Mount Manager under key and returns what the caller still
// has to mount. The caller must deactivate key once they are unmounted.
//
// It returns an error matching [cerrdefs.IsNotImplemented] when the snapshotter skips Mount
// Manager, when containerd's Mount Manager has no mounts to handle, or when containerd has no
// Mount Manager, so the caller performs them all and has nothing to deactivate.
func (m *refCountMounter) activate(ctx context.Context, key string, mounts []mount.Mount, opts ...mount.ActivateOpt) ([]mount.Mount, error) {
	if m.skipMountManager() {
		return nil, cerrdefs.ErrNotImplemented
	}

	info, err := m.mm.Activate(ctx, key, mounts, opts...)
	if cerrdefs.IsAlreadyExists(err) {
		// Reuse the activation that already exists under key, e.g. one left by a
		// daemon that died between activating and mounting.
		info, err = m.mm.Info(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("failed to get info on active mounts %s: %w", key, err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to activate mounts %s: %w", key, err)
	}

	if len(info.System) == 0 && len(info.Active) > 0 {
		// containerd's Mount Manager performed every mount under its own directory
		// (e.g. a view of a single erofs layer). Let the caller bind the last mount
		// point, which holds the resulting filesystem, to its target directory.
		return []mount.Mount{{
			Type:    "bind",
			Source:  info.Active[len(info.Active)-1].MountPoint,
			Options: []string{"rbind"},
		}}, nil
	}
	// containerd's Mount Manager performed the mounts that need it (e.g. the erofs
	// layers) and left the rest (e.g. the overlay over them) to the caller.
	return info.System, nil
}

// deactivate treats a missing activation or a missing Mount Manager as already deactivated.
func (m *refCountMounter) deactivate(ctx context.Context, key string) error {
	err := m.mm.Deactivate(ctx, key)
	if cerrdefs.IsNotFound(err) || cerrdefs.IsNotImplemented(err) {
		return nil
	}
	return err
}

// deactivateTarget deactivates the rootfs mounts of target once it is unmounted.
func (m *refCountMounter) deactivateTarget(ctx context.Context, target string) error {
	if m.skipMountManager() {
		return nil
	}
	if isMounted(target) {
		log.G(ctx).WithField("target", target).Warn("keeping mounts active: it is still mounted")
		return nil
	}
	if err := m.deactivate(ctx, m.activationKey(target)); err != nil {
		return fmt.Errorf("failed to deactivate mounts of %s: %w", target, err)
	}

	return nil
}

func (m *refCountMounter) activationKey(target string) string {
	return "moby-" + m.base.snapshotter + "-sn-" + filepath.Base(target)
}

func (m *refCountMounter) Unmount(target string) error {
	if count := m.rc.Decrement(target); count > 0 {
		return nil
	}

	m.locker.Lock(target)
	defer m.locker.Unlock(target)

	if err := unmount(target); err != nil {
		log.G(context.TODO()).Debugf("Failed to unmount %s: %v", target, err)
	}

	// Remove the mount point even if deactivation fails, and return that error
	// so the caller sees the leftover activation.
	deactivateErr := m.deactivateTarget(context.TODO(), target)

	if err := os.Remove(target); err != nil {
		log.G(context.TODO()).WithError(err).WithField("dir", target).Error("failed to remove mount temp dir")
	}

	return deactivateErr
}

func (m *refCountMounter) Mounted(containerID string) (string, error) {
	mounted, err := m.base.Mounted(containerID)
	if err != nil || mounted == "" {
		return mounted, err
	}

	target := m.base.target(containerID)

	// Check if the refcount is non-zero.
	m.rc.Increment(target)
	if m.rc.Decrement(target) > 0 {
		return mounted, nil
	}

	return "", nil
}

type mounter struct {
	home        string
	snapshotter string
	idMap       user.IdentityMapping
}

func (m mounter) Mount(mounts []mount.Mount, containerID string) (string, error) {
	target := m.target(containerID)

	uid, gid := m.idMap.RootPair()
	if err := user.MkdirAllAndChown(filepath.Dir(target), 0o710, os.Getuid(), gid); err != nil {
		return "", err
	}
	if err := user.MkdirAllAndChown(target, 0o710, uid, gid); err != nil {
		return "", err
	}

	return target, mount.All(mounts, target)
}

func (m mounter) Unmount(target string) error {
	return unmount(target)
}

func (m mounter) Mounted(containerID string) (string, error) {
	target := m.target(containerID)

	mounted, err := mountinfo.Mounted(target)
	if err != nil || !mounted {
		return "", err
	}
	return target, nil
}

func (m mounter) target(containerID string) string {
	return filepath.Join(m.home, "rootfs", m.snapshotter, containerID)
}
