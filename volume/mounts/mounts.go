package mounts // import "github.com/docker/docker/volume/mounts"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"syscall"

	"github.com/containerd/log"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/internal/safepath"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/volume"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
)

// MountPoint is the intersection point between a volume and a container. It
// specifies which volume is to be used and where inside a container it should
// be mounted.
//
// Note that this type is embedded in `container.Container` object and persisted to disk.
// Changes to this struct need to by synced with on disk state.
type MountPoint struct {
	// Source is the source path of the mount.
	// E.g. `mount --bind /foo /bar`, `/foo` is the `Source`.
	Source string
	// Destination is the path relative to the container root (`/`) to the mount point
	// It is where the `Source` is mounted to
	Destination string
	// RW is set to true when the mountpoint should be mounted as read-write
	RW bool
	// Name is the name reference to the underlying data defined by `Source`
	// e.g., the volume name
	Name string
	// Driver is the volume driver used to create the volume (if it is a volume)
	Driver string
	// Type of mount to use, see `Type<foo>` definitions in github.com/docker/docker/api/types/mount
	Type mounttypes.Type `json:",omitempty"`
	// Volume is the volume providing data to this mountpoint.
	// This is nil unless `Type` is set to `TypeVolume`
	Volume volume.Volume `json:"-"`

	// Mode is the comma separated list of options supplied by the user when creating
	// the bind/volume mount.
	// Note Mode is not used on Windows
	Mode string `json:"Relabel,omitempty"` // Originally field was `Relabel`"

	// Propagation describes how the mounts are propagated from the host into the
	// mount point, and vice-versa.
	// See https://www.kernel.org/doc/Documentation/filesystems/sharedsubtree.txt
	// Note Propagation is not used on Windows
	Propagation mounttypes.Propagation `json:",omitempty"` // Mount propagation string

	// Specifies if data should be copied from the container before the first mount
	// Use a pointer here so we can tell if the user set this value explicitly
	// This allows us to error out when the user explicitly enabled copy but we can't copy due to the volume being populated
	CopyData bool `json:"-"`
	// ID is the opaque ID used to pass to the volume driver.
	// This should be set by calls to `Mount` and unset by calls to `Unmount`
	ID string `json:",omitempty"`

	// Spec is a copy of the API request that created this mount.
	Spec mounttypes.Mount

	// Some bind mounts should not be automatically created.
	// (Some are auto-created for backwards-compatibility)
	// This is checked on the API but setting this here prevents race conditions.
	// where a bind dir existed during validation was removed before reaching the setup code.
	SkipMountpointCreation bool

	// Track usage of this mountpoint
	// Specifically needed for containers which are running and calls to `docker cp`
	// because both these actions require mounting the volumes.
	active int

	// SafePaths created by Setup that should be cleaned up before unmounting
	// the volume.
	safePaths []*safepath.SafePath
}

// Cleanup frees resources used by the mountpoint and cleans up all the paths
// returned by Setup that hasn't been cleaned up by the caller.
func (m *MountPoint) Cleanup(ctx context.Context) error {
	if m.Volume == nil || m.ID == "" {
		return nil
	}

	logger := log.G(ctx).WithFields(log.Fields{"active": m.active, "id": m.ID})

	// TODO: Remove once the real bug is fixed: https://github.com/moby/moby/issues/46508
	if m.active == 0 {
		logger.Error("An attempt to decrement a zero mount count")
		logger.Error(string(debug.Stack()))
		return nil
	}

	for _, p := range m.safePaths {
		if !p.IsValid() {
			continue
		}

		err := p.Close(ctx)
		base, sub := p.SourcePath()
		log.G(ctx).WithFields(log.Fields{
			"error":         err,
			"path":          p.Path(),
			"sourceBase":    base,
			"sourceSubpath": sub,
		}).Warn("cleaning up SafePath that hasn't been cleaned up by the caller")
	}

	if err := m.Volume.Unmount(m.ID); err != nil {
		return errors.Wrapf(err, "error unmounting volume %s", m.Volume.Name())
	}

	m.active--
	logger.Debug("MountPoint.Cleanup Decrement active count")

	if m.active == 0 {
		m.ID = ""
	}
	return nil
}

// Setup sets up a mount point by either mounting the volume if it is
// configured, or creating the source directory if supplied.
// The, optional, checkFun parameter allows doing additional checking
// before creating the source directory on the host.
//
// The returned path can be a temporary path, caller is responsible to
// call the returned cleanup function as soon as the path is not needed.
// Cleanup doesn't unmount the underlying volumes (if any), it only
// frees up the resources that were needed to guarantee that the path
// still points to the same target (to avoid TOCTOU attack).
//
// Cleanup function doesn't need to be called when error is returned.
func (m *MountPoint) Setup(ctx context.Context, mountLabel string, rootIDs idtools.Identity, checkFun func(m *MountPoint) error) (path string, cleanup func(context.Context) error, retErr error) {
	if m.SkipMountpointCreation {
		return m.Source, noCleanup, nil
	}

	defer func() {
		if retErr != nil || !label.RelabelNeeded(m.Mode) {
			return
		}

		sourcePath, err := filepath.EvalSymlinks(path)
		if err != nil {
			path = ""
			retErr = errors.Wrapf(err, "error evaluating symlinks from mount source %q", m.Source)
			if cleanupErr := cleanup(ctx); cleanupErr != nil {
				log.G(ctx).WithError(cleanupErr).Warn("failed to cleanup after error")
			}
			cleanup = noCleanup
			return
		}
		err = label.Relabel(sourcePath, mountLabel, label.IsShared(m.Mode))
		if err != nil && !errors.Is(err, syscall.ENOTSUP) {
			path = ""
			retErr = errors.Wrapf(err, "error setting label on mount source '%s'", sourcePath)
			if cleanupErr := cleanup(ctx); cleanupErr != nil {
				log.G(ctx).WithError(cleanupErr).Warn("failed to cleanup after error")
			}
			cleanup = noCleanup
		}
	}()

	if m.Volume != nil {
		id := m.ID
		if id == "" {
			id = stringid.GenerateRandomID()
		}
		volumePath, err := m.Volume.Mount(id)
		if err != nil {
			return "", noCleanup, errors.Wrapf(err, "error while mounting volume '%s'", m.Source)
		}

		m.ID = id
		clean := noCleanup
		if m.Spec.VolumeOptions != nil && m.Spec.VolumeOptions.Subpath != "" {
			subpath := m.Spec.VolumeOptions.Subpath

			safePath, err := safepath.Join(ctx, volumePath, subpath)
			if err != nil {
				if err := m.Volume.Unmount(id); err != nil {
					log.G(ctx).WithError(err).Error("failed to unmount after safepath.Join failed")
				}
				return "", noCleanup, err
			}
			m.safePaths = append(m.safePaths, safePath)
			log.G(ctx).Debugf("mounting (%s|%s) via %s", volumePath, subpath, safePath.Path())

			clean = safePath.Close
			volumePath = safePath.Path()
		}

		m.active++
		return volumePath, clean, nil
	}

	if len(m.Source) == 0 {
		return "", noCleanup, fmt.Errorf("Unable to setup mount point, neither source nor volume defined")
	}

	if m.Type == mounttypes.TypeBind {
		// Before creating the source directory on the host, invoke checkFun if it's not nil. One of
		// the use case is to forbid creating the daemon socket as a directory if the daemon is in
		// the process of shutting down.
		if checkFun != nil {
			if err := checkFun(m); err != nil {
				return "", noCleanup, err
			}
		}

		// idtools.MkdirAllNewAs() produces an error if m.Source exists and is a file (not a directory)
		// also, makes sure that if the directory is created, the correct remapped rootUID/rootGID will own it
		if err := idtools.MkdirAllAndChownNew(m.Source, 0o755, rootIDs); err != nil {
			if perr, ok := err.(*os.PathError); ok {
				if perr.Err != syscall.ENOTDIR {
					return "", noCleanup, errors.Wrapf(err, "error while creating mount source path '%s'", m.Source)
				}
			}
		}
	}
	return m.Source, noCleanup, nil
}

func (m *MountPoint) LiveRestore(ctx context.Context) error {
	if m.Volume == nil {
		log.G(ctx).Debug("No volume to restore")
		return nil
	}

	lrv, ok := m.Volume.(volume.LiveRestorer)
	if !ok {
		log.G(ctx).WithField("volume", m.Volume.Name()).Debugf("Volume does not support live restore: %T", m.Volume)
		return nil
	}

	id := m.ID
	if id == "" {
		id = stringid.GenerateRandomID()
	}

	if err := lrv.LiveRestoreVolume(ctx, id); err != nil {
		return errors.Wrapf(err, "error while restoring volume '%s'", m.Source)
	}

	m.ID = id
	m.active++
	return nil
}

// Path returns the path of a volume in a mount point.
func (m *MountPoint) Path() string {
	if m.Volume != nil {
		return m.Volume.Path()
	}
	return m.Source
}

func errInvalidMode(mode string) error {
	return errors.Errorf("invalid mode: %v", mode)
}

func errInvalidSpec(spec string) error {
	return errors.Errorf("invalid volume specification: '%s'", spec)
}

// noCleanup is a no-op cleanup function.
func noCleanup(_ context.Context) error {
	return nil
}
