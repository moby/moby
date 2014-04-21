package fs

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
		uptime, startTime float64
		paramData         = make(map[string]float64)
		cpuTotal          = 0.0
	)

	path, err := d.path("cpuacct")
	if err != nil {
		return paramData, err
	}
	f, err := os.Open(filepath.Join(path, "cpuacct.stat"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		t, v, err := getCgroupParamKeyValue(sc.Text())
		if err != nil {
			return paramData, err
		}
		// set the raw data in map
		paramData[t] = v
		cpuTotal += v
	}

	if uptime, err = s.getUptime(); err != nil {
		return nil, err
	}
	if startTime, err = s.getProcStarttime(d); err != nil {
		return nil, err
	}
	//paramData["percentage"] = 100.0 * ((cpuTotal/100.0)/uptime - (startTime / 100))
	paramData["percentage"] = cpuTotal / (uptime - (startTime / 100))

	return paramData, nil
}

func (s *cpuacctGroup) getUptime() (float64, error) {
	f, err := os.Open("/proc/uptime")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.Fields(string(data))[0], 64)
}

func (s *cpuacctGroup) getProcStarttime(d *data) (float64, error) {
	rawStart, err := system.GetProcessStartTime(d.pid)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(rawStart, 64)
}
