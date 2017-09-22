package cgroups

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
)

const nanosecondsInSecond = 1000000000

var clockTicks = getClockTicks()

func NewCpuacct(root string) *cpuacctController {
	return &cpuacctController{
		root: filepath.Join(root, string(Cpuacct)),
	}
}

type cpuacctController struct {
	root string
}

func (c *cpuacctController) Name() Name {
	return Cpuacct
}

func (c *cpuacctController) Path(path string) string {
	return filepath.Join(c.root, path)
}

func (c *cpuacctController) Stat(path string, stats *Metrics) error {
	user, kernel, err := c.getUsage(path)
	if err != nil {
		return err
	}
	total, err := readUint(filepath.Join(c.Path(path), "cpuacct.usage"))
	if err != nil {
		return err
	}
	percpu, err := c.percpuUsage(path)
	if err != nil {
		return err
	}
	stats.CPU.Usage.Total = total
	stats.CPU.Usage.User = user
	stats.CPU.Usage.Kernel = kernel
	stats.CPU.Usage.PerCPU = percpu
	return nil
}

func (c *cpuacctController) percpuUsage(path string) ([]uint64, error) {
	var usage []uint64
	data, err := ioutil.ReadFile(filepath.Join(c.Path(path), "cpuacct.usage_percpu"))
	if err != nil {
		return nil, err
	}
	for _, v := range strings.Fields(string(data)) {
		u, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, err
		}
		usage = append(usage, u)
	}
	return usage, nil
}

func (c *cpuacctController) getUsage(path string) (user uint64, kernel uint64, err error) {
	statPath := filepath.Join(c.Path(path), "cpuacct.stat")
	data, err := ioutil.ReadFile(statPath)
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) != 4 {
		return 0, 0, fmt.Errorf("%q is expected to have 4 fields", statPath)
	}
	for _, t := range []struct {
		index int
		name  string
		value *uint64
	}{
		{
			index: 0,
			name:  "user",
			value: &user,
		},
		{
			index: 2,
			name:  "system",
			value: &kernel,
		},
	} {
		if fields[t.index] != t.name {
			return 0, 0, fmt.Errorf("expected field %q but found %q in %q", t.name, fields[t.index], statPath)
		}
		v, err := strconv.ParseUint(fields[t.index+1], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		*t.value = v
	}
	return (user * nanosecondsInSecond) / clockTicks, (kernel * nanosecondsInSecond) / clockTicks, nil
}
