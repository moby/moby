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
	cpuCount   = int64(runtime.NumCPU())
	clockTicks = int64(system.GetClockTicks())
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

func (s *cpuacctGroup) Stats(d *data) (map[string]int64, error) {
	var (
		startCpu, lastCpu, startSystem, lastSystem, startUsage, lastUsage int64
		percentage                                                        int64
		paramData                                                         = make(map[string]int64)
	)
	path, err := d.path("cpuacct")
	if startCpu, err = s.getCpuUsage(d, path); err != nil {
		return nil, cgroups.ErrStatsNotFound
	}
	if startSystem, err = s.getSystemCpuUsage(d); err != nil {
		return nil, cgroups.ErrStatsNotFound
	}
	startUsageTime := time.Now()
	if startUsage, err = getCgroupParamInt(path, "cpuacct.usage"); err != nil {
		return nil, err
	}
	// sample for 100ms
	time.Sleep(100 * time.Millisecond)
	if lastCpu, err = s.getCpuUsage(d, path); err != nil {
		return nil, cgroups.ErrStatsNotFound
	}
	if lastSystem, err = s.getSystemCpuUsage(d); err != nil {
		return nil, cgroups.ErrStatsNotFound
	}
	usageSampleDuration := time.Since(startUsageTime)
	if lastUsage, err = getCgroupParamInt(path, "cpuacct.usage"); err != nil {
		return nil, err
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
	paramData["percentage"] = percentage

	// Delta usage is in nanoseconds of CPU time so get the usage (in cores) over the sample time.
	paramData["usage"] = deltaUsage / usageSampleDuration.Nanoseconds()
	return paramData, nil
}

// TODO(vmarmol): Use cgroups stats.
func (s *cpuacctGroup) getProcStarttime(d *data) (int64, error) {
	rawStart, err := system.GetProcessStartTime(d.pid)
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(rawStart)
	if err != nil {
		return 0, err
	}
	return int64(v), nil
}

func (s *cpuacctGroup) getSystemCpuUsage(d *data) (int64, error) {

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

			var total int64
			for _, i := range parts[1:8] {
				v, err := strconv.ParseInt(i, 10, 64)
				if err != nil {
					return 0.0, fmt.Errorf("Unable to convert value %s to int: %s", i, err)
				}
				total += int64(v)
			}
			return total, nil
		default:
			continue
		}
	}
	return 0, fmt.Errorf("invalid stat format")
}

func (s *cpuacctGroup) getCpuUsage(d *data, path string) (int64, error) {
	cpuTotal := int64(0)
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
		cpuTotal += int64(v)
	}
	return cpuTotal, nil
}
