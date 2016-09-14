package volume

import (
	"fmt"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/components/volume/drivers"
	"github.com/docker/docker/components/volume/local"
	"github.com/docker/docker/components/volume/store"
	"github.com/docker/docker/components/volume/types"
	"github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/component"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/volume"
)

type backend struct {
	volumes       *store.VolumeStore
	eventsService component.Events
}

func (b *backend) init(context *component.Context, config component.Config) error {
	b.eventsService = context.Events

	fsConfig := config.Filesystem
	volumesDriver, err := local.New(fsConfig.Root, fsConfig.UID, fsConfig.GID)
	if err != nil {
		return err
	}

	if !drivers.Register(volumesDriver, volumesDriver.Name()) {
		return fmt.Errorf("local volume driver could not be registered")
	}
	b.volumes, err = store.New(fsConfig.Root)
	return err
}

// Inspect looks up a volume by name. An error is returned if the volume
// cannot be found.
func (b *backend) Inspect(name string) (*types.Volume, error) {
	v, err := b.volumes.Get(name)
	if err != nil {
		return nil, err
	}
	apiV := volumeToAPIType(v)
	apiV.Mountpoint = v.Path()
	apiV.Status = v.Status()
	return apiV, nil
}

// GetWithRef returns a volume
func (b *backend) GetWithRef(name, driver, ref string) (volume.Volume, error) {
	return b.volumes.GetWithRef(name, driver, ref)
}

// VolumeCreate creates a volume with the specified name, driver, and opts
// This is called directly from the remote API
func (b *backend) Create(name, driverName, ref string, opts, labels map[string]string) (volume.Volume, error) {
	if name == "" {
		name = stringid.GenerateNonCryptoID()
	}

	v, err := b.volumes.CreateWithRef(name, driverName, ref, opts, labels)
	if err != nil {
		if err.IsNameConflict() {
			return nil, fmt.Errorf("A volume named %s already exists. Choose a different volume name.", name)
		}
		return nil, err
	}

	b.event(v.Name(), "create", map[string]string{"driver": v.DriverName()})
	return v, nil
}

var acceptedVolumeFilterTags = map[string]bool{
	"dangling": true,
	"name":     true,
	"driver":   true,
	"label":    true,
}

// List volumes, using the filter to restrict the range of volumes returned.
func (b *backend) List(filter string) ([]*types.Volume, []string, error) {
	var (
		volumesOut []*types.Volume
	)
	volFilters, err := filters.FromParam(filter)
	if err != nil {
		return nil, nil, err
	}

	if err := volFilters.Validate(acceptedVolumeFilterTags); err != nil {
		return nil, nil, err
	}

	volumes, warnings, err := b.volumes.List()
	if err != nil {
		return nil, nil, err
	}

	filterVolumes, err := b.filterVolumes(volumes, volFilters)
	if err != nil {
		return nil, nil, err
	}
	for _, v := range filterVolumes {
		apiV := volumeToAPIType(v)
		if vv, ok := v.(interface {
			CachedPath() string
		}); ok {
			apiV.Mountpoint = vv.CachedPath()
		} else {
			apiV.Mountpoint = v.Path()
		}
		volumesOut = append(volumesOut, apiV)
	}
	return volumesOut, warnings, nil
}

// filterVolumes filters volume list according to user specified filter
// and returns user chosen volumes
func (b *backend) filterVolumes(vols []volume.Volume, filter filters.Args) ([]volume.Volume, error) {
	// if filter is empty, return original volume list
	if filter.Len() == 0 {
		return vols, nil
	}

	var retVols []volume.Volume
	for _, vol := range vols {
		if filter.Include("name") {
			if !filter.Match("name", vol.Name()) {
				continue
			}
		}
		if filter.Include("driver") {
			if !filter.Match("driver", vol.DriverName()) {
				continue
			}
		}
		if filter.Include("label") {
			v, ok := vol.(volume.LabeledVolume)
			if !ok {
				continue
			}
			if !filter.MatchKVList("label", v.Labels()) {
				continue
			}
		}
		retVols = append(retVols, vol)
	}
	danglingOnly := false
	if filter.Include("dangling") {
		if filter.ExactMatch("dangling", "true") || filter.ExactMatch("dangling", "1") {
			danglingOnly = true
		} else if !filter.ExactMatch("dangling", "false") && !filter.ExactMatch("dangling", "0") {
			return nil, fmt.Errorf("Invalid filter 'dangling=%s'", filter.Get("dangling"))
		}
		retVols = b.volumes.FilterByUsed(retVols, !danglingOnly)
	}
	return retVols, nil
}

// RemoveByName a volume by name.
// If the volume is referenced by a container it is not removed
func (b *backend) RemoveByName(name string, force bool) error {
	err := b.remove(name)
	if err == nil || force {
		b.volumes.Purge(name)
		return nil
	}
	return err
}

func (b *backend) remove(name string) error {
	v, err := b.volumes.Get(name)
	if err != nil {
		return err
	}

	if err := b.volumes.Remove(v); err != nil {
		if err.IsInUse() {
			err := fmt.Errorf("Unable to remove volume, volume still in use: %v", err)
			return errors.NewRequestConflictError(err)
		}
		return fmt.Errorf("Error while removing volume %s: %v", name, err)
	}
	b.event(v.Name(), "destroy", map[string]string{"driver": v.DriverName()})
	return nil
}

func (b *backend) event(volumeID, action string, attributes map[string]string) {
	actor := events.Actor{ID: volumeID, Attributes: attributes}
	b.eventsService.Log(action, events.VolumeEventType, actor)
}

// volumeToAPIType converts a volume.Volume to the type used by the remote API
func volumeToAPIType(v volume.Volume) *types.Volume {
	tv := &types.Volume{Name: v.Name(), Driver: v.DriverName()}
	if v, ok := v.(volume.LabeledVolume); ok {
		tv.Labels = v.Labels()
	}

	if v, ok := v.(volume.ScopedVolume); ok {
		tv.Scope = v.Scope()
	}
	return tv
}

// Remove a volume
func (b *backend) Remove(v volume.Volume) error {
	return b.volumes.Remove(v)
}

// Dereference removes a reference to a volume in the store
func (b *backend) Dereference(v volume.Volume, ref string) {
	b.volumes.Dereference(v, ref)
}

// DriverList returns the list of registered drivers
func (b *backend) DriverList() []string {
	return drivers.GetDriverList()
}

func (b *backend) MigrateVolume17(id, path string) error {
	return migrateVolume17(id, path)
}

var _ types.VolumeComponent = &backend{}
