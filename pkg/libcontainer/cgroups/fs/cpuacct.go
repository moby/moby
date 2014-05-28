package fs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dotcloud/docker/pkg/libcontainer/cgroups"
	"github.com/dotcloud/docker/pkg/system"
)

var (
	cpuCount   = uint64(runtime.NumCPU())
	clockTicks = uint64(system.GetClockTicks())
)

type cpuacctGroup struct {
}

func (s *cpuacctGroup) Set(d *data) error {
	// we just want to join this group even though we don't set anything
	if _, err := d.join("cpuacct"); err != nil && err != cgroups.ErrNotFound {
		return err
	}
	return nil
}

func (s *cpuacctGroup) Remove(d *data) error {
	return removePath(d.path("cpuacct"))
}

func (s *cpuacctGroup) GetStats(d *data, stats *cgroups.Stats) error {
	var (
		startCpu, lastCpu, startSystem, lastSystem, startUsage, lastUsage uint64
		percentage                                                        uint64
	)
	path, err := d.path("cpuacct")
	if startCpu, err = s.getCpuUsage(d, path); err != nil {
		return err
	}
	if startSystem, err = s.getSystemCpuUsage(d); err != nil {
		return err
	}
	startUsageTime := time.Now()
	if startUsage, err = getCgroupParamInt(path, "cpuacct.usage"); err != nil {
		return err
	}
	// sample for 100ms
	time.Sleep(100 * time.Millisecond)
	if lastCpu, err = s.getCpuUsage(d, path); err != nil {
		return err
	}
	if lastSystem, err = s.getSystemCpuUsage(d); err != nil {
		return err
	}
	usageSampleDuration := time.Since(startUsageTime)
	if lastUsage, err = getCgroupParamInt(path, "cpuacct.usage"); err != nil {
		return err
	}

	var (
		deltaProc   = lastCpu - startCpu
		deltaSystem = lastSystem - startSystem
		deltaUsage  = lastUsage - startUsage
	)
	if deltaSystem > 0.0 {
		percentage = ((deltaProc / deltaSystem) * clockTicks) * cpuCount
	}
	// NOTE: a percentage over 100% is valid for POSIX because that means the
	// processes is using multiple cores
	stats.CpuStats.CpuUsage.PercentUsage = percentage
	// Delta usage is in nanoseconds of CPU time so get the usage (in cores) over the sample time.
	stats.CpuStats.CpuUsage.CurrentUsage = deltaUsage / uint64(usageSampleDuration.Nanoseconds())
	return nil
}

// TODO(vmarmol): Use cgroups stats.
func (s *cpuacctGroup) getSystemCpuUsage(d *data) (uint64, error) {

	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		switch parts[0] {
		case "cpu":
			if len(parts) < 8 {
				return 0, fmt.Errorf("invalid number of cpu fields")
			}

			var total uint64
			for _, i := range parts[1:8] {
				v, err := strconv.ParseUint(i, 10, 64)
				if err != nil {
					return 0.0, fmt.Errorf("Unable to convert value %s to int: %s", i, err)
				}
				total += v
			}
			return total, nil
		default:
			continue
		}
	}
	return 0, fmt.Errorf("invalid stat format")
}

func (s *cpuacctGroup) getCpuUsage(d *data, path string) (uint64, error) {
	cpuTotal := uint64(0)
	f, err := os.Open(filepath.Join(path, "cpuacct.stat"))
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		_, v, err := getCgroupParamKeyValue(sc.Text())
		if err != nil {
			return 0, err
		}
		// set the raw data in map
		cpuTotal += v
	}
	return cpuTotal, nil
}
