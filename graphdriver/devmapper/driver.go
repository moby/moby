// +build linux,amd64

package devmapper

import (
	"fmt"
	"github.com/dotcloud/docker/graphdriver"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"path"
	"sync"
)

func init() {
	graphdriver.Register("devicemapper", Init)
}

// Placeholder interfaces, to be replaced
// at integration.

// End of placeholder interfaces.

type Driver struct {
	*DeviceSet
	home       string
	sync.Mutex // Protects concurrent modification to active
	active     map[string]int
}

var Init = func(home string) (graphdriver.Driver, error) {
	deviceSet, err := NewDeviceSet(home, true)
	if err != nil {
		return nil, err
	}
	d := &Driver{
		DeviceSet: deviceSet,
		home:      home,
		active:    make(map[string]int),
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

func (d *Driver) Create(id, parent string) error {
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

	return nil
}

func (d *Driver) Remove(id string) error {
	// Protect the d.active from concurrent access
	d.Lock()
	defer d.Unlock()

	if d.active[id] != 0 {
		utils.Errorf("Warning: removing active id %s\n", id)
	}

	mp := path.Join(d.home, "mnt", id)
	if err := d.unmount(id, mp); err != nil {
		return err
	}
	return d.DeviceSet.RemoveDevice(id)
}

func (d *Driver) Get(id string) (string, error) {
	// Protect the d.active from concurrent access
	d.Lock()
	defer d.Unlock()

	count := d.active[id]

	mp := path.Join(d.home, "mnt", id)
	if count == 0 {
		if err := d.mount(id, mp); err != nil {
			return "", err
		}
	}

	d.active[id] = count + 1

	return path.Join(mp, "rootfs"), nil
}

func (d *Driver) Put(id string) {
	// Protect the d.active from concurrent access
	d.Lock()
	defer d.Unlock()

	if count := d.active[id]; count > 1 {
		d.active[id] = count - 1
	} else {
		mp := path.Join(d.home, "mnt", id)
		d.unmount(id, mp)
		delete(d.active, id)
	}
}

func (d *Driver) mount(id, mountPoint string) error {
	// Create the target directories if they don't exist
	if err := osMkdirAll(mountPoint, 0755); err != nil && !osIsExist(err) {
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

func (d *Driver) unmount(id, mountPoint string) error {
	// If mountpoint is not mounted, do nothing
	if mounted, err := Mounted(mountPoint); err != nil {
		return fmt.Errorf("Error checking mountpoint: %s", err)
	} else if !mounted {
		return nil
	}
	// Unmount the device
	return d.DeviceSet.UnmountDevice(id, mountPoint, true)
}

func (d *Driver) Exists(id string) bool {
	return d.Devices[id] != nil
}
