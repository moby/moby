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
		paramData                                  = make(map[string]float64)
		percentage                                 = 0.0
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
		percentage = ((deltaProc / deltaSystem) * 100.0) * float64(runtime.NumCPU())
	}
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
	total := 0.0
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		txt := sc.Text()
		if strings.Index(txt, "cpu") == 0 {
			parts := strings.Fields(txt)
			partsLength := len(parts)
			if partsLength != 11 {
				return 0.0, fmt.Errorf("Unable to parse cpu usage: expected 11 fields ; received %d", partsLength)
			}
			for _, i := range parts[1:10] {
				val, err := strconv.ParseFloat(i, 64)
				if err != nil {
					return 0.0, fmt.Errorf("Unable to convert value to float: %s", err)
				}
				total += val
			}
			break
		}
	}
	return total, nil
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
