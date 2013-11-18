package devmapper

import (
	"fmt"
	"github.com/dotcloud/docker/graphdriver"
	"os"
	"path"
)

func init() {
	graphdriver.Register("devicemapper", Init)
}

// Placeholder interfaces, to be replaced
// at integration.

// End of placeholder interfaces.

type Driver struct {
	*DeviceSet
	home string
}

func Init(home string) (graphdriver.Driver, error) {
	deviceSet, err := NewDeviceSet(home, true)
	if err != nil {
		return nil, err
	}
	d := &Driver{
		DeviceSet: deviceSet,
		home:      home,
	}
	return d, nil
}

func (d *Driver) String() string {
	return "devicemapper"
}

func (d *Driver) Status() [][2]string {
	s := d.DeviceSet.Status()

	status := [][2]string{
		{"Pool Name", s.PoolName},
		{"Data file", s.DataLoopback},
		{"Metadata file", s.MetadataLoopback},
		{"Data Space Used", fmt.Sprintf("%.1f Mb", float64(s.Data.Used)/(1024*1024))},
		{"Data Space Total", fmt.Sprintf("%.1f Mb", float64(s.Data.Total)/(1024*1024))},
		{"Metadata Space Used", fmt.Sprintf("%.1f Mb", float64(s.Metadata.Used)/(1024*1024))},
		{"Metadata Space Total", fmt.Sprintf("%.1f Mb", float64(s.Metadata.Total)/(1024*1024))},
	}
	return status
}

func (d *Driver) Cleanup() error {
	return d.DeviceSet.Shutdown()
}

func (d *Driver) Create(id string, parent string) error {
	return d.DeviceSet.AddDevice(id, parent)
}

func (d *Driver) Remove(id string) error {
	return d.DeviceSet.RemoveDevice(id)
}

func (d *Driver) Get(id string) (string, error) {
	mp := path.Join(d.home, "mnt", id)
	if err := d.mount(id, mp); err != nil {
		return "", err
	}
	return mp, nil
}

func (d *Driver) Size(id string) (int64, error) {
	return -1, fmt.Errorf("Not implemented")
}

func (d *Driver) mount(id, mountPoint string) error {
	// Create the target directories if they don't exist
	if err := os.MkdirAll(mountPoint, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	// If mountpoint is already mounted, do nothing
	if mounted, err := Mounted(mountPoint); err != nil {
		return fmt.Errorf("Error checking mountpoint: %s", err)
	} else if mounted {
		return nil
	}
	// Mount the device
	return d.DeviceSet.MountDevice(id, mountPoint, false)
}
