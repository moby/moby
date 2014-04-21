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

	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/system"
)

var (
	cpuCount   = float64(runtime.NumCPU())
	clockTicks = float64(system.GetClockTicks())
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

func (s *cpuacctGroup) Stats(d *data) (map[string]float64, error) {
	var (
		startCpu, lastCpu, startSystem, lastSystem float64
		percentage                                 float64
		paramData                                  = make(map[string]float64)
	)
	path, err := d.path("cpuacct")
	if startCpu, err = s.getCpuUsage(d, path); err != nil {
		return nil, err
	}
	if startSystem, err = s.getSystemCpuUsage(d); err != nil {
		return nil, err
	}
	// sample for 100ms
	time.Sleep(100 * time.Millisecond)
	if lastCpu, err = s.getCpuUsage(d, path); err != nil {
		return nil, err
	}
	if lastSystem, err = s.getSystemCpuUsage(d); err != nil {
		return nil, err
	}

	var (
		deltaProc   = lastCpu - startCpu
		deltaSystem = lastSystem - startSystem
	)
	if deltaSystem > 0.0 {
		percentage = ((deltaProc / deltaSystem) * clockTicks) * cpuCount
	}
	// NOTE: a percentage over 100% is valid for POSIX because that means the
	// processes is using multiple cores
	paramData["percentage"] = percentage
	return paramData, nil
}

func (s *cpuacctGroup) getProcStarttime(d *data) (float64, error) {
	rawStart, err := system.GetProcessStartTime(d.pid)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(rawStart, 64)
}

func (s *cpuacctGroup) getSystemCpuUsage(d *data) (float64, error) {

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

			var total float64
			for _, i := range parts[1:8] {
				v, err := strconv.ParseFloat(i, 64)
				if err != nil {
					return 0.0, fmt.Errorf("Unable to convert value %s to float: %s", i, err)
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

func (s *cpuacctGroup) getCpuUsage(d *data, path string) (float64, error) {
	cpuTotal := 0.0
	f, err := os.Open(filepath.Join(path, "cpuacct.stat"))
	if err != nil {
		return 0.0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		_, v, err := getCgroupParamKeyValue(sc.Text())
		if err != nil {
			return 0.0, err
		}
		// set the raw data in map
		cpuTotal += v
	}
	return cpuTotal, nil
}
