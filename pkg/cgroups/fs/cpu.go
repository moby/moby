package fs

import (
	"bufio"
	"os"
	"path/filepath"
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
	if d.c.CpuPeriod != 0 {
		if err := writeFile(dir, "cpu.cfs_period_us", strconv.FormatInt(d.c.CpuPeriod, 10)); err != nil {
			return err
		}
	}
	if d.c.CpuQuota != 0 {
		if err := writeFile(dir, "cpu.cfs_quota_us", strconv.FormatInt(d.c.CpuQuota, 10)); err != nil {
			return err
		}
	}
	return nil
}

func (s *cpuGroup) Remove(d *data) error {
	return removePath(d.path("cpu"))
}

func (s *cpuGroup) Stats(d *data) (map[string]float64, error) {
	paramData := make(map[string]float64)
	path, err := d.path("cpu")
	if err != nil {
		return nil, err
	}

	f, err := os.Open(filepath.Join(path, "cpu.stat"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		t, v, err := getCgroupParamKeyValue(sc.Text())
		if err != nil {
			return nil, err
		}
		paramData[t] = v
	}
	return paramData, nil
}
