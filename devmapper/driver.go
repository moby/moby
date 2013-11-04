package devmapper

import (
	"fmt"
	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/graphdriver"
	"os"
	"path"
)

func init() {
	graphdriver.Register("devicemapper", Init)
}

// Placeholder interfaces, to be replaced
// at integration.

type Image interface {
	ID() string
	Parent() (Image, error)
	Path() string
}

type Change interface {
}

// End of placeholder interfaces.

type Driver struct {
	*DeviceSet
	home string
}

func Init(home string) (graphdriver.Driver, error) {
	d := &Driver{
		DeviceSet: NewDeviceSet(home),
		home:      home,
	}
	if err := d.DeviceSet.ensureInit(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Driver) Cleanup() error {
	return d.DeviceSet.Shutdown()
}

func (d *Driver) OnCreate(img Image, layer archive.Archive) error {
	// Determine the source of the snapshot (parent id or init device)
	var parentID string
	if parent, err := img.Parent(); err != nil {
		return err
	} else if parent != nil {
		parentID = parent.ID()
	}
	// Create the device for this image by snapshotting source
	if err := d.DeviceSet.AddDevice(img.ID(), parentID); err != nil {
		return err
	}
	// Mount the device in rootfs
	mp := d.mountpoint(img.ID())
	if err := os.MkdirAll(mp, 0700); err != nil {
		return err
	}
	if err := d.DeviceSet.MountDevice(img.ID(), mp, false); err != nil {
		return err
	}
	// Apply the layer as a diff
	if layer != nil {
		if err := archive.ApplyLayer(mp, layer); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) OnRemove(img Image) error {
	id := img.ID()
	if err := d.DeviceSet.RemoveDevice(id); err != nil {
		return fmt.Errorf("Unable to remove device for %v: %v", id, err)
	}
	return nil
}

func (d *Driver) mountpoint(id string) string {
	if d.home == "" {
		return ""
	}
	return path.Join(d.home, "mnt", id)
}

func (d *Driver) Changes(img *Image, dest string) ([]Change, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (d *Driver) Layer(img *Image, dest string) (archive.Archive, error) {
	return nil, fmt.Errorf("Not implemented")
}
