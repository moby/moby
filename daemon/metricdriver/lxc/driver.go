package lxc

import (
	"fmt"
	"github.com/dotcloud/docker/daemon/metricdriver"
	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	MEMORY        = "memory"
	MEMORY_STAT   = "memory.stat"
	CPUACCT       = "cpuacct"
	CPUACCT_USAGE = "cpuacct.usage"
	RSS           = "rss"
	REFRESH_TIME  = 1
)

var (
	previousCpuUsages = make(map[string]int64)
	cpuUsages         = make(map[string]int64)
)

type Driver struct {
}

func init() {
	metricdriver.Register("lxc", Init)
	go monitor()
}

func Init() (metricdriver.Driver, error) {
	return &Driver{}, nil
}

func (driver *Driver) Get(id string) (*metricdriver.Metric, error) {

	metric := metricdriver.NewMetric()

	if memory, err := getMemoryInfo(id); err != nil {
		return nil, err
	} else {
		metric.Memory = memory
	}

	if cpu, err := getCpuInfo(id); err != nil {
		return nil, err
	} else {
		metric.Cpu = cpu
	}

	return metric, nil
}

func getCpuInfo(id string) (*metricdriver.Cpu, error) {
	var cpu metricdriver.Cpu
	cpuUsage, cpuUsagesExist := cpuUsages[id]
	if !cpuUsagesExist {
		return nil, fmt.Errorf("Can not get cpu usage for %s", id)
	}

	previousCpuUsage, previousCpuUsageExist := previousCpuUsages[id]
	if !previousCpuUsageExist {
		return nil, fmt.Errorf("Can not get cpu usage for %s", id)
	}

	cpu.NumOfCPU = runtime.NumCPU()
	cpu.LoadAverage = float64(cpuUsage-previousCpuUsage) / float64(1e9)

	return &cpu, nil
}

func getMemoryInfo(id string) (*metricdriver.Memory, error) {
	if output, err := readSubsystemFile(id, MEMORY, MEMORY_STAT); err != nil {
		return nil, err
	} else {
		memory, err := parseMemoryStatFile(output)
		if err != nil {
			return nil, err
		}
		return memory, nil
	}
}

func getCpuAcctUsage(id string) (int64, error) {
	if output, err := readSubsystemFile(id, CPUACCT, CPUACCT_USAGE); err != nil {
		return -1, err
	} else {
		cpu, err := parseCpuAcctUsageFile(output)
		if err != nil {
			return -1, err
		}
		return cpu, nil
	}
}

func parseMemoryStatFile(content string) (*metricdriver.Memory, error) {
	var memory metricdriver.Memory
	for _, line := range strings.Split(string(content), "\n") {
		if len(line) == 0 {
			continue
		}
		pair := strings.Fields(line)
		if len(pair) != 2 {
			return nil, fmt.Errorf("Invalid pair line '%s'.", pair)
		}
		if pair[0] == RSS {
			value, err := strconv.Atoi(pair[1])
			if err != nil {
				return nil, fmt.Errorf("Invalid rss '%s': %s", value, err)
			}
			memory.Rss = int64(value)
			break
		}
	}
	return &memory, nil
}

func parseCpuAcctUsageFile(content string) (int64, error) {
	content = strings.TrimSuffix(string(content), "\n")
	if len(content) == 0 {
		return -1, fmt.Errorf("Invalid line '%s'", content)
	}

	value, err := strconv.Atoi(content)
	if err != nil {
		return -1, err
	} else {
		return int64(value), nil
	}
}

func readSubsystemFile(id, subsystem, filename string) (string, error) {
	cgroupRoot, err := cgroups.FindCgroupMountpoint(subsystem)
	if err != nil {
		return "", err
	}

	cgroupDir, err := cgroups.GetThisCgroupDir(subsystem)
	if err != nil {
		return "", err
	}

	path := filepath.Join(cgroupRoot, cgroupDir, id, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = filepath.Join(cgroupRoot, cgroupDir, "lxc", id, filename)
	}

	output, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func containers(subsystem string) (map[string]bool, error) {

	containers := make(map[string]bool)

	cgroupRoot, err := cgroups.FindCgroupMountpoint(subsystem)
	if err != nil {
		return containers, err
	}

	cgroupDir, err := cgroups.GetThisCgroupDir(subsystem)
	if err != nil {
		return containers, err
	}

	dir := filepath.Join(cgroupRoot, cgroupDir, "lxc")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		dir = filepath.Join(cgroupRoot, cgroupDir)
	}

	if fileList, err := ioutil.ReadDir(dir); err != nil {
		return containers, err
	} else {
		for _, fileInfo := range fileList {
			if fileInfo.IsDir() && len(fileInfo.Name()) == 64 {
				containers[fileInfo.Name()] = true
			}
		}
	}

	return containers, nil
}

func monitor() {
	tick := time.NewTicker(REFRESH_TIME * time.Second)
	var mutex = &sync.Mutex{}
	for {
		<-tick.C
		mutex.Lock()

		containers, err := containers(CPUACCT)

		for k, v := range cpuUsages {
			if _, exist := containers[k]; exist {
				previousCpuUsages[k] = v
			} else {
				delete(previousCpuUsages, k)
				delete(cpuUsages, k)
			}
		}

		if err != nil {
			utils.Errorf("Error read containers from cgroup directory: %s", err)
			continue
		}

		for id, _ := range containers {
			n, err := getCpuAcctUsage(id)
			if err != nil {
				utils.Errorf("Error read cpuacct usage: %s", err)
			}
			cpuUsages[id] = n
		}

		mutex.Unlock()
	}
}
