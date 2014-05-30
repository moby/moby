package fs

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/dotcloud/docker/pkg/libcontainer/cgroups"
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

func (s *cpuGroup) GetStats(d *data, stats *cgroups.Stats) error {
	path, err := d.path("cpu")
	if err != nil {
		return err
	}

	f, err := os.Open(filepath.Join(path, "cpu.stat"))
	if err != nil {
		if pathErr, ok := err.(*os.PathError); ok && pathErr.Err == syscall.ENOENT {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		t, v, err := getCgroupParamKeyValue(sc.Text())
		if err != nil {
			return err
		}
		switch t {
		case "nr_periods":
			stats.CpuStats.ThrottlingData.Periods = v

		case "nr_throttled":
			stats.CpuStats.ThrottlingData.ThrottledPeriods = v

		case "throttled_time":
			stats.CpuStats.ThrottlingData.ThrottledTime = v
		}
	}
	return nil
}
