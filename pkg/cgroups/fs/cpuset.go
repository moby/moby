package fs

import (
	"os"
)

type cpusetGroup struct {
}

func (s *cpusetGroup) Set(d *data) error {
	// we don't want to join this cgroup unless it is specified
	if d.c.CpusetCpus != "" {
		dir, err := d.join("cpuset")
		if err != nil && d.c.CpusetCpus != "" {
			return err
		}
		defer func() {
			if err != nil {
				os.RemoveAll(dir)
			}
		}()

		if err := writeFile(dir, "cpuset.cpus", d.c.CpusetCpus); err != nil {
			return err
		}
	}
	return nil
}

func (s *cpusetGroup) Remove(d *data) error {
	return removePath(d.path("cpuset"))
}

func (s *cpusetGroup) Stats(d *data) (map[string]float64, error) {
	return nil, ErrNotSupportStat
}
