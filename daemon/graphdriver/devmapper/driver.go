// +build linux

package devmapper

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/log"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/units"
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

func Init(home string, options []string) (graphdriver.Driver, error) {
	deviceSet, err := NewDeviceSet(home, true, options)
	if err != nil {
		return nil, err
	}

	if err := graphdriver.MakePrivate(home); err != nil {
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
		{"Pool Blocksize", fmt.Sprintf("%s", units.HumanSize(int64(s.SectorSize)))},
		{"Data file", s.DataLoopback},
		{"Metadata file", s.MetadataLoopback},
		{"Data Space Used", fmt.Sprintf("%s", units.HumanSize(int64(s.Data.Used)))},
		{"Data Space Total", fmt.Sprintf("%s", units.HumanSize(int64(s.Data.Total)))},
		{"Metadata Space Used", fmt.Sprintf("%s", units.HumanSize(int64(s.Metadata.Used)))},
		{"Metadata Space Total", fmt.Sprintf("%s", units.HumanSize(int64(s.Metadata.Total)))},
	}
	return status
}

func (d *Driver) Cleanup() error {
	err := d.DeviceSet.Shutdown()

	if err2 := mount.Unmount(d.home); err == nil {
		err = err2
	}

	return err
}

func (d *Driver) Create(id, parent string) error {
	if err := d.DeviceSet.AddDevice(id, parent); err != nil {
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

func (d *Driver) Get(id, mountLabel string) (string, error) {
	mp := path.Join(d.home, "mnt", id)

	// Create the target directories if they don't exist
	if err := os.MkdirAll(mp, 0755); err != nil && !os.IsExist(err) {
		return "", err
	}

	// Mount the device
	if err := d.DeviceSet.MountDevice(id, mp, mountLabel); err != nil {
		return "", err
	}

	rootFs := path.Join(mp, "rootfs")
	if err := os.MkdirAll(rootFs, 0755); err != nil && !os.IsExist(err) {
		d.DeviceSet.UnmountDevice(id)
		return "", err
	}

	idFile := path.Join(mp, "id")
	if _, err := os.Stat(idFile); err != nil && os.IsNotExist(err) {
		// Create an "id" file with the container/image id in it to help reconscruct this in case
		// of later problems
		if err := ioutil.WriteFile(idFile, []byte(id), 0600); err != nil {
			d.DeviceSet.UnmountDevice(id)
			return "", err
		}
	}

	return rootFs, nil
}

func (d *Driver) Put(id string) {
	if err := d.DeviceSet.UnmountDevice(id); err != nil {
		log.Errorf("Warning: error unmounting device %s: %s", id, err)
	}
}

func (d *Driver) Exists(id string) bool {
	return d.DeviceSet.HasDevice(id)
}
