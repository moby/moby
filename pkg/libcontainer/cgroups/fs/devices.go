package fs

import (
	"fmt"

	"github.com/dotcloud/docker/pkg/libcontainer/devices"
)

type devicesGroup struct {
}

func (s *devicesGroup) Set(d *data) error {
	dir, err := d.join("devices")
	if err != nil {
		return err
	}

	if !d.c.UnlimitedDeviceAccess {
		if err := writeFile(dir, "devices.deny", "a"); err != nil {
			return err
		}

		for _, dev := range d.c.AllowedDevices {
			deviceAllowString := fmt.Sprintf("%c %s:%s %s", dev.Type, devices.GetDeviceNumberString(dev.MajorNumber), devices.GetDeviceNumberString(dev.MinorNumber), dev.CgroupPermissions)
			if err := writeFile(dir, "devices.allow", deviceAllowString); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *devicesGroup) Remove(d *data) error {
	return removePath(d.path("devices"))
}

func (s *devicesGroup) Stats(d *data) (map[string]int64, error) {
	return nil, ErrNotSupportStat
}
