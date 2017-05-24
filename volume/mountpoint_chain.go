package volume

import (
	"fmt"
	"os"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/volume/mountpoint"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// MountPointChain uses a list of mount point middleware to interpose
// on mount point attachment and detachment
type MountPointChain struct {
	mu         sync.Mutex
	middleware []mountpoint.Middleware
}

// NewMountPointChain creates a new Chain with a slice of plugin names.
func NewMountPointChain(names []string, pg plugingetter.PluginGetter) (*MountPointChain, error) {
	mountpoint.SetPluginGetter(pg)
	plugins, err := mountpoint.NewPlugins(names)
	if err != nil {
		return nil, err
	}
	middleware := make([]mountpoint.Middleware, len(plugins))
	for i := range plugins {
		middleware[i] = plugins[i]
	}
	return &MountPointChain{
		middleware: middleware,
	}, nil
}

// AttachMounts runs a list of mount attachments through a mount point middleware chain
func (c *MountPointChain) AttachMounts(image *container.Config, container *container.Config, id string, mounts []*MountPoint) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var mountPointClock int

	for _, middleware := range c.middleware {
		var selectedMounts []*MountPoint
		patterns := middleware.Patterns()

		mountPointClock++

		// select mounts for this middleware
		for _, mount := range mounts {
			for _, pattern := range patterns {
				if mountpoint.PatternMatches(pattern, middlewareMountPointOfMountPoint(image, container, mount)) {
					selectedMounts = append(selectedMounts, mount)
					break
				}
			}
		}

		if len(selectedMounts) > 0 {
			// send attachment request to the middleware
			var pmounts []*mountpoint.MountPoint
			for _, selectedMount := range selectedMounts {
				pmounts = append(pmounts, middlewareMountPointOfMountPoint(image, container, selectedMount))
			}
			request := &mountpoint.AttachRequest{id, pmounts}
			response, err := middleware.MountPointAttach(request)
			if err != nil {
				return c.unwindAttachOnErr(middleware.Name(), id, mounts, err)
			}
			if !response.Success {
				return c.unwindAttachOnErr(middleware.Name(), id, mounts, errors.New(response.Err))
			}

			// annotate the mount points with the applied middleware
			for k, attachment := range response.Attachments {
				if k >= len(selectedMounts) {
					break
				}
				if attachment.Attach {
					selectedMounts[k].PushMiddleware(middleware, attachment.EmergencyDetach, attachment.Changes, mountPointClock)
				}
			}
		}
	}

	return nil
}

// DetachMounts detaches mounts from a mount point middlware chain
func (c *MountPointChain) DetachMounts(container string, mounts map[string]*MountPoint) error {
	var list []*MountPoint
	for _, mp := range mounts {
		list = append(list, mp)
	}
	return unwind(container, list)
}

// unwindAttachOnErr will clean up previous attachments if an error
// occurs during attachment
func (c *MountPointChain) unwindAttachOnErr(middlewareName, container string, mounts []*MountPoint, err error) (ret error) {
	defer func() {
		ret = errors.Wrap(ret, "middleware "+middlewareName+" failed with error")
	}()

	e := unwind(container, mounts)
	if e != nil {
		return errors.Wrap(err, fmt.Sprintf("%s", e))
	}

	ret = err
	return ret
}

// unwind is used to detach all middleware participating in a
// container's mount points. Middleware are detached in the opposite
// order that they were attached. Because the middlware chain can
// change dynamically, the applied mount point stack for a container
// changes during setup, not all middleware apply to all mounts, and
// middleware application is local to each mount point, we use a counter
// (clock) to keep track of the order that middlware were applied in the
// mount point applied middleware stacks.
func unwind(container string, mounts []*MountPoint) error {
	var err error
	var middleware *mountpoint.Middleware
	moreToUnwind := true

	for moreToUnwind {
		maxClock := 0
		moreToUnwind = false

		// find the clock value of the next middleware to detach
		for _, mount := range mounts {
			maxClock = max(mount.TopClock(), maxClock)
		}

		if maxClock > 0 {
			moreToUnwind = true

			appliedMiddleware := []*AppliedMountPointMiddleware{}
			detachingMounts := []*MountPoint{}
			for _, mount := range mounts {
				// if the top middleware on this mount isn't the next to
				// detach, skip this mount
				if mount.TopClock() < maxClock {
					continue
				}

				appliedM := mount.PopMiddleware()
				if appliedM != nil {
					// if we don't yet have the middleware object, get it
					// otherwise, check that the name of the applied
					// middleware for this mount is indeed the same as our
					// middleware object
					if middleware == nil {
						m, e := appliedM.Middleware()
						if e != nil {
							errString := fmt.Sprintf("unwind middleware retrieval error: \"%s\"", e)
							return stackError(err, errString)
						}
						middleware = m
					} else if (*middleware).Name() != appliedM.Name {
						return fmt.Errorf("middleware inconsistency %s != %s", (*middleware).Name(), appliedM.Name)
					}

					appliedMiddleware = append(appliedMiddleware, appliedM)
					detachingMounts = append(detachingMounts, mount)
				}
			}
			// send the middleware the mount point detach request and deal
			// with both protocol errors and detachment errors
			request := &mountpoint.DetachRequest{container}
			response, e := (*middleware).MountPointDetach(request)
			if e != nil {
				logrus.Warnf("mount point middleware %s detach API error: \"%s\"", (*middleware).Name(), e)
				var successfulDetach bool
				successfulDetach, err = emergencyDetach(err, detachingMounts, appliedMiddleware)
				if !successfulDetach {
					return err
				}
			} else if !response.Success {
				errString := fmt.Sprintf("unwind detach middleware %s error: \"%s\"", (*middleware).Name(), response.Err)
				err = stackError(err, errString)
				if !response.Recoverable {
					return err
				}
			}
		}
		middleware = nil
	}
	return err
}

func emergencyDetach(error error, mounts []*MountPoint, appliedMiddleware []*AppliedMountPointMiddleware) (success bool, err error) {
	success = true

	effectiveSource := func(mount *MountPoint, appliedM *AppliedMountPointMiddleware) string {
		if appliedM.Changes.EffectiveSource != "" {
			return appliedM.Changes.EffectiveSource
		}
		return mount.EffectiveSource()
	}

	for i, appliedM := range appliedMiddleware {
		if len(appliedM.Emergency) == 0 {
			success = false
			errString := fmt.Sprintf("after mount point middleware %s detach API error, mount point %s fatally lacks emergency detach sequence", appliedM.Name, effectiveSource(mounts[i], appliedM))
			err = stackError(err, errString)
		}

		for _, action := range appliedM.Emergency {
			if action.Error != "" {
				err = stackError(err, action.Error)
			}
			if action.Fatal != "" {
				err = stackError(err, action.Fatal)
				success = false
			}
			if action.Remove != "" {
				success, err = emergencyDetachRemove(success, err, action.Remove)
			}
			if action.Unmount != "" {
				success, err = emergencyDetachUnmount(success, err, action.Unmount)
			}
			if action.Warning != "" {
				logrus.Warnf("emergency mount point middleware %s detach from %s: %s", appliedM.Name, effectiveSource(mounts[i], appliedM), action.Warning)
			}
		}
	}

	return success, err
}

func emergencyDetachRemove(success bool, err error, path string) (bool, error) {
	if e := os.Remove(path); e != nil {
		errString := fmt.Sprintf("error removing path %s during emergency mount point detach: %s", path, e)
		return false, stackError(err, errString)
	}

	return success, err
}

func emergencyDetachUnmount(success bool, err error, path string) (bool, error) {
	if e := mount.Unmount(path); e != nil {
		errString := fmt.Sprintf("error unmounting path %s during emergency mount point detach: %s", path, e)
		return false, stackError(err, errString)
	}

	return success, err
}

// stackError will wrap err in errString if err is an error or create
// a new error from errString if err is nil
func stackError(err error, errString string) error {
	if err == nil {
		return errors.New(errString)
	}
	return errors.Wrap(err, errString)
}

// max is the greater integer of a and b. Seriously?
func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

// SetPlugins sets the mount point plugins in the chain
func (c *MountPointChain) SetPlugins(names []string) (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var plugins []mountpoint.Plugin
	if plugins, err = mountpoint.NewPlugins(names); err != nil {
		return err
	}
	c.middleware = make([]mountpoint.Middleware, len(plugins))
	for i := range plugins {
		c.middleware[i] = plugins[i]
	}
	return nil
}

// AppendPluginIfMissing appends the mount point plugin named to the
// end of the chain if it isn't already in the chain
func (c *MountPointChain) AppendPluginIfMissing(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, p := range c.middleware {
		if p.PluginName() == name {
			return nil
		}
	}

	plugin, err := mountpoint.NewMountPointPlugin(name)
	if err != nil {
		return err
	}
	c.middleware = append(c.middleware, plugin)
	return nil
}

// DisableMountPointPlugin removes the mount point plugin from the chain
func (c *MountPointChain) DisableMountPointPlugin(name string) {
	c.DisableMountPointMiddleware("plugin:" + name)
}

// DisableMountPointMiddleware removes the mount point middleware from the chain
func (c *MountPointChain) DisableMountPointMiddleware(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// There may still be mounts which are relying on it during tear
	// down
	var middleware []mountpoint.Middleware
	for _, m := range c.middleware {
		if m.Name() == name {
			m.Destroy()
		} else {
			middleware = append(middleware, m)
		}
	}
	c.middleware = middleware
}

func mountPointTypeOfAPIType(t mounttypes.Type) mountpoint.Type {
	var typ mountpoint.Type
	switch t {
	case mounttypes.TypeBind:
		typ = mountpoint.TypeBind
	case mounttypes.TypeVolume:
		typ = mountpoint.TypeVolume
	}
	return typ
}

func middlewareMountPointOfMountPoint(image *container.Config, container *container.Config, mp *MountPoint) *mountpoint.MountPoint {
	typ := mountPointTypeOfAPIType(mp.Type)
	var labels map[string]string
	var driverOptions map[string]string
	if mp.Spec.VolumeOptions != nil {
		labels = mp.Spec.VolumeOptions.Labels
		driverOptions = mp.Spec.VolumeOptions.DriverConfig.Options
	}
	var scope mountpoint.Scope
	var options map[string]string
	if v, ok := mp.Volume.(DetailedVolume); ok {
		scope = mountpoint.Scope(v.Scope())
		options = v.Options()
	}
	v := mountpoint.Volume{
		Name:          mp.Name,
		Driver:        mp.Driver,
		ID:            mp.ID,
		Labels:        labels,
		DriverOptions: driverOptions,
		Scope:         scope,
		Options:       options,
	}
	c := mountpoint.Container{
		Labels: container.Labels,
	}
	i := mountpoint.Image{
		Labels: image.Labels,
	}
	return &mountpoint.MountPoint{
		Source:                mp.Source,
		EffectiveSource:       mp.EffectiveSource(),
		Destination:           mp.Destination,
		ReadOnly:              !mp.RW,
		Type:                  typ,
		Mode:                  mp.Mode,
		Propagation:           mp.Propagation,
		CreateSourceIfMissing: mp.CreateSourceIfMissing,
		Consistency:           mp.Consistency,
		EffectiveConsistency:  mp.EffectiveConsistency(),
		AppliedMiddleware:     MountPointAppliedMiddlewareOfAppliedMountPointMiddleware(mp.AppliedMiddleware),
		Volume:                v,
		Container:             c,
		Image:                 i,
	}
}

// MountPointAppliedMiddlewareOfAppliedMountPointMiddleware converts a
// stack of the runtime type AppliedMountPointMiddleware into the API
// type types.MountPointAppliedMiddleware
func MountPointAppliedMiddlewareOfAppliedMountPointMiddleware(middleware []AppliedMountPointMiddleware) []types.MountPointAppliedMiddleware {
	ms := []types.MountPointAppliedMiddleware{}
	for _, m := range middleware {
		ms = append(ms, types.MountPointAppliedMiddleware{
			Name:    m.Name,
			Changes: m.Changes,
		})
	}

	return ms
}
