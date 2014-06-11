package fs

import "github.com/docker/libcontainer/cgroups"

type devicesGroup struct {
}

func (s *devicesGroup) Set(d *data) error {
	dir, err := d.join("devices")
	if err != nil {
		return err
	}

	if !d.c.AllowAllDevices {
		if err := writeFile(dir, "devices.deny", "a"); err != nil {
			return err
		}

		for _, dev := range d.c.AllowedDevices {
			if err := writeFile(dir, "devices.allow", dev.GetCgroupAllowString()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *devicesGroup) Remove(d *data) error {
	return removePath(d.path("devices"))
}

func (s *devicesGroup) GetStats(d *data, stats *cgroups.Stats) error {
	return nil
}
