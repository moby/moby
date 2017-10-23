package cgroups

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func NewCpu(root string) *cpuController {
	return &cpuController{
		root: filepath.Join(root, string(Cpu)),
	}
}

type cpuController struct {
	root string
}

func (c *cpuController) Name() Name {
	return Cpu
}

func (c *cpuController) Path(path string) string {
	return filepath.Join(c.root, path)
}

func (c *cpuController) Create(path string, resources *specs.LinuxResources) error {
	if err := os.MkdirAll(c.Path(path), defaultDirPerm); err != nil {
		return err
	}
	if cpu := resources.CPU; cpu != nil {
		for _, t := range []struct {
			name   string
			ivalue *int64
			uvalue *uint64
		}{
			{
				name:   "rt_period_us",
				uvalue: cpu.RealtimePeriod,
			},
			{
				name:   "rt_runtime_us",
				ivalue: cpu.RealtimeRuntime,
			},
			{
				name:   "shares",
				uvalue: cpu.Shares,
			},
			{
				name:   "cfs_period_us",
				uvalue: cpu.Period,
			},
			{
				name:   "cfs_quota_us",
				ivalue: cpu.Quota,
			},
		} {
			var value []byte
			if t.uvalue != nil {
				value = []byte(strconv.FormatUint(*t.uvalue, 10))
			} else if t.ivalue != nil {
				value = []byte(strconv.FormatInt(*t.ivalue, 10))
			}
			if value != nil {
				if err := ioutil.WriteFile(
					filepath.Join(c.Path(path), fmt.Sprintf("cpu.%s", t.name)),
					value,
					defaultFilePerm,
				); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *cpuController) Update(path string, resources *specs.LinuxResources) error {
	return c.Create(path, resources)
}

func (c *cpuController) Stat(path string, stats *Metrics) error {
	f, err := os.Open(filepath.Join(c.Path(path), "cpu.stat"))
	if err != nil {
		return err
	}
	defer f.Close()
	// get or create the cpu field because cpuacct can also set values on this struct
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		key, v, err := parseKV(sc.Text())
		if err != nil {
			return err
		}
		switch key {
		case "nr_periods":
			stats.CPU.Throttling.Periods = v
		case "nr_throttled":
			stats.CPU.Throttling.ThrottledPeriods = v
		case "throttled_time":
			stats.CPU.Throttling.ThrottledTime = v
		}
	}
	return nil
}
