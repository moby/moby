// +build linux,amd64

package devmapper

import (
	"fmt"
	"github.com/dotcloud/docker/daemon/graphdriver"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
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

var Init = func(home string) (graphdriver.Driver, error) {
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

func (d *Driver) Create(id, parent string, mountLabel string) error {
	if err := d.DeviceSet.AddDevice(id, parent); err != nil {
		return err
	}
	mp := path.Join(d.home, "mnt", id)
	if err := d.mount(id, mp); err != nil {
		return err
	}

	if err := osMkdirAll(path.Join(mp, "rootfs"), 0755); err != nil && !osIsExist(err) {
		return err
	}

	// Create an "id" file with the container/image id in it to help reconscruct this in case
	// of later problems
	if err := ioutil.WriteFile(path.Join(mp, "id"), []byte(id), 0600); err != nil {
		return err
	}

	// We float this reference so that the next Get call can
	// steal it, so we don't have to unmount
	if err := d.DeviceSet.UnmountDevice(id, UnmountFloat); err != nil {
		return err
	}

	return nil
}

func (d *Driver) Remove(id string) error {
	if !d.DeviceSet.HasDevice(id) {
		// Consider removing a non-existing device a no-op
		// This is useful to be able to progress on container removal
		// if the underlying device has gone away due to earlier errors
		return nil
	}

	// Sink the float from create in case no Get() call was made
	if err := d.DeviceSet.UnmountDevice(id, UnmountSink); err != nil {
		return err
	}
	// This assumes the device has been properly Get/Put:ed and thus is unmounted
	if err := d.DeviceSet.DeleteDevice(id); err != nil {
		return err
	}

	mp := path.Join(d.home, "mnt", id)
	if err := os.RemoveAll(mp); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (d *Driver) Get(id string) (string, error) {
	mp := path.Join(d.home, "mnt", id)
	if err := d.mount(id, mp); err != nil {
		return "", err
	}

	return path.Join(mp, "rootfs"), nil
}

func (d *Driver) Put(id string) {
	if err := d.DeviceSet.UnmountDevice(id, UnmountRegular); err != nil {
		utils.Errorf("Warning: error unmounting device %s: %s\n", id, err)
	}
}

func (d *Driver) mount(id, mountPoint string) error {
	// Create the target directories if they don't exist
	if err := osMkdirAll(mountPoint, 0755); err != nil && !osIsExist(err) {
		return err
	}
	// Mount the device
	return d.DeviceSet.MountDevice(id, mountPoint, "")
}

func (d *Driver) Exists(id string) bool {
	return d.Devices[id] != nil
}
