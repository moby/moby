package fs

import (
	"bufio"
	"fmt"
	"io/ioutil"
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

const nanosecondsInSecond = 1000000000

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
		startCpu, lastCpu, startSystem, lastSystem, startUsage, lastUsage, kernelModeUsage, userModeUsage, percentage uint64
	)
	path, err := d.path("cpuacct")
	if kernelModeUsage, userModeUsage, err = s.getCpuUsage(d, path); err != nil {
		return err
	}
	startCpu = kernelModeUsage + userModeUsage
	if startSystem, err = s.getSystemCpuUsage(d); err != nil {
		return err
	}
	startUsageTime := time.Now()
	if startUsage, err = getCgroupParamInt(path, "cpuacct.usage"); err != nil {
		return err
	}
	// sample for 100ms
	time.Sleep(100 * time.Millisecond)
	if kernelModeUsage, userModeUsage, err = s.getCpuUsage(d, path); err != nil {
		return err
	}
	lastCpu = kernelModeUsage + userModeUsage
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
	percpuUsage, err := s.getPercpuUsage(path)
	if err != nil {
		return err
	}
	stats.CpuStats.CpuUsage.PercpuUsage = percpuUsage
	stats.CpuStats.CpuUsage.UsageInKernelmode = (kernelModeUsage * nanosecondsInSecond) / clockTicks
	stats.CpuStats.CpuUsage.UsageInUsermode = (userModeUsage * nanosecondsInSecond) / clockTicks
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

func (s *cpuacctGroup) getCpuUsage(d *data, path string) (uint64, uint64, error) {
	kernelModeUsage := uint64(0)
	userModeUsage := uint64(0)
	data, err := ioutil.ReadFile(filepath.Join(path, "cpuacct.stat"))
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) != 4 {
		return 0, 0, fmt.Errorf("Failure - %s is expected to have 4 fields", filepath.Join(path, "cpuacct.stat"))
	}
	if userModeUsage, err = strconv.ParseUint(fields[1], 10, 64); err != nil {
		return 0, 0, err
	}
	if kernelModeUsage, err = strconv.ParseUint(fields[3], 10, 64); err != nil {
		return 0, 0, err
	}

	return kernelModeUsage, userModeUsage, nil
}

func (s *cpuacctGroup) getPercpuUsage(path string) ([]uint64, error) {
	percpuUsage := []uint64{}
	data, err := ioutil.ReadFile(filepath.Join(path, "cpuacct.usage_percpu"))
	if err != nil {
		return percpuUsage, err
	}
	for _, value := range strings.Fields(string(data)) {
		value, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return percpuUsage, fmt.Errorf("Unable to convert param value to uint64: %s", err)
		}
		percpuUsage = append(percpuUsage, value)
	}
	return percpuUsage, nil
}
