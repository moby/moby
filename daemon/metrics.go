package daemon

import (
	"fmt"
	"os"
	"strings"

	"io/ioutil"
	"path/filepath"

	"github.com/docker/docker/engine"
	"github.com/docker/libcontainer/cgroups"
)

var metricInfoTable map[string][]string

func init() {
	metricInfoTable = map[string][]string{
		"CpuUsage":       {"cpuacct", "usage"},
		"MemoryUsage":    {"memory", "usage_in_bytes"},
		"MemoryMaxUsage": {"memory", "max_usage_in_bytes"},
		"MemoryLimit":    {"memory", "limit_in_bytes"},
	}
}

func (daemon *Daemon) ContainerMetrics(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("usage: %s NAME", job.Name)
	}
	name := job.Args[0]
	if container := daemon.Get(name); container != nil {
		container.Lock()
		defer container.Unlock()

		out := &engine.Env{}
		out.Set("Id", container.ID)
		out.Set("Image", container.Image)
		out.Set("Name", container.Name)
		out.SetJson("State", container.State)

		metrics := map[string]string{}
		for metricKey, cgroupInfo := range metricInfoTable {
			cgroupSystem, cgroupKey := cgroupInfo[0], cgroupInfo[1]
			value, err := cgSubsystemValue(container.ID, cgroupSystem, cgroupKey)
			if err != nil {
				metrics[metricKey] = err.Error()
			} else {
				metrics[metricKey] = value
			}
		}
		out.SetJson("Metrics", metrics)

		if _, err := out.WriteTo(job.Stdout); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	}
	return job.Errorf("No such container: %s", name)
}

func cgSubsystemValue(id, subsystem, key string) (string, error) {
	cgDir, err := cgSubsystemDir(id, subsystem)
	if err != nil {
		return "", err
	}
	data, err := ioutil.ReadFile(filepath.Join(cgDir, fmt.Sprintf("%s.%s", subsystem, key)))
	if err != nil {
		return "", err
	}
	return strings.Trim(string(data), "\r\n"+string(0)), nil
}

func cgSubsystemDir(id, subsystem string) (string, error) {
	cgroupRoot, err := cgroups.FindCgroupMountpoint(subsystem)
	if err != nil {
		return "", err
	}

	cgroupDir, err := cgroups.GetThisCgroupDir(subsystem)
	if err != nil {
		return "", err
	}

	// With more recent lxc versions use, cgroup will be in lxc/, we'll search in bot
	dirnames := []string{
		filepath.Join(cgroupRoot, cgroupDir, id),
		filepath.Join(cgroupRoot, cgroupDir, "docker", id),
		filepath.Join(cgroupRoot, cgroupDir, "lxc", id),
	}
	for i := range dirnames {
		if _, err := os.Stat(dirnames[i]); err == nil {
			return dirnames[i], nil
		}
	}

	return "", fmt.Errorf("Error: cgroup subsystem '%s' directory not found for container '%s'", subsystem, id)
}
