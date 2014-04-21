package fs

import (
	"strconv"
)

type cpuGroup struct {
}

func (s *cpuGroup) Set(d *data) error {
	// We always want to join the cpu group, to allow fair cpu scheduling
	// on a container basis
	dir, err := d.join("cpu")
	if err != nil {
		return err
	}
	if d.c.CpuShares != 0 {
		if err := writeFile(dir, "cpu.shares", strconv.FormatInt(d.c.CpuShares, 10)); err != nil {
			return err
		}
	}
	return nil
}

func (s *cpuGroup) Remove(d *data) error {
	return removePath(d.path("cpu"))
}

func (s *cpuGroup) Stats(d *data) (map[string]float64, error) {
	// we can reuse the cpuacct subsystem to get the cpu stats
	sys := subsystems["cpuacct"]
	return sys.Stats(d)
}
