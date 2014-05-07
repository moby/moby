package fs

import (
	"os"
)

type devicesGroup struct {
}

func (s *devicesGroup) Set(d *data) error {
	dir, err := d.join("devices")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(dir)
		}
	}()

	if !d.c.DeviceAccess {
		if err := writeFile(dir, "devices.deny", "a"); err != nil {
			return err
		}

		allow := []string{
			// allow mknod for any device
			"c *:* m",
			"b *:* m",

			// /dev/null, zero, full
			"c 1:3 rwm",
			"c 1:5 rwm",
			"c 1:7 rwm",

			// consoles
			"c 5:1 rwm",
			"c 5:0 rwm",
			"c 4:0 rwm",
			"c 4:1 rwm",

			// /dev/urandom,/dev/random
			"c 1:9 rwm",
			"c 1:8 rwm",

			// /dev/pts/ - pts namespaces are "coming soon"
			"c 136:* rwm",
			"c 5:2 rwm",

			// tuntap
			"c 10:200 rwm",
		}

		for _, val := range allow {
			if err := writeFile(dir, "devices.allow", val); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *devicesGroup) Remove(d *data) error {
	return removePath(d.path("devices"))
}

func (s *devicesGroup) Stats(d *data) (map[string]float64, error) {
	return nil, ErrNotSupportStat
}
