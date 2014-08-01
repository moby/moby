package fs

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/docker/libcontainer/cgroups"
)

var (
	AccessaibleSubsystems = map[string][]string{
		"cpu":     []string{"cpu.shares", "cpu.cfs_period_us", "cpu.cfs_quota_us"},
		"cpuset":  []string{"cpuset.cpus"},
		"memory":  []string{"memory.limit_in_bytes", "memory.soft_limit_in_bytes", "memory.memsw.limit_in_bytes"},
		"freezer": []string{"freezer.state"},
	}
	ErrCanNotAccess = errors.New("this subsystem can not be accessed")
)

func Set(id, driver, subsystem, value string) error {
	path, err := getPath(id, driver, subsystem)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, []byte(value), 0700)
}

func Get(id, driver, subsystem string) (string, error) {
	path, err := getPath(id, driver, subsystem)
	if err != nil {
		return "", err
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}

func findGroup(subsystem string) (string, error) {
	for group, subsystems := range AccessaibleSubsystems {
		for _, s := range subsystems {
			if s == subsystem {
				return group, nil
			}
		}
	}
	return "", ErrCanNotAccess
}

func getPath(id, driver, subsystem string) (string, error) {
	cgroupRoot, err := cgroups.FindCgroupMountpoint("cpu")
	if err != nil {
		return "", err
	}

	cgroupRoot = filepath.Dir(cgroupRoot)
	if _, err := os.Stat(cgroupRoot); err != nil {
		return "", fmt.Errorf("cgroups fs not found")
	}

	group, err := findGroup(subsystem)
	if err != nil {
		return "", err
	}

	initPath, err := cgroups.GetInitCgroupDir(group)
	if err != nil {
		return "", err
	}

	path := path.Join(cgroupRoot, group, initPath, driver, id, subsystem)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("%s not found", path)
	}
	return path, nil
}

func GetStatsFix(c *cgroups.Cgroup) (*cgroups.Stats, error) {
	var (
		subsystems = map[string]subsystem{
			"devices":    &devicesGroup{},
			"memory":     &memoryGroup{},
			"cpu":        &cpuGroup{},
			"cpuset":     &cpusetGroup{},
			"cpuacct":    &cpuacctGroup{},
			"perf_event": &perfEventGroup{},
		}
	)
	stats := cgroups.NewStats()

	d, err := getCgroupData(c, 0)
	if err != nil {
		return nil, fmt.Errorf("getting CgroupData %s", err)
	}

	for sysName, sys := range subsystems {
		// Don't fail if a cgroup hierarchy was not found.
		if err := sys.GetStats(d, stats); err != nil && err != cgroups.ErrNotFound {
			return nil, fmt.Errorf("getting stats for system %q %s", sysName, err)
		}
	}

	return stats, nil
}
