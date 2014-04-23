package fs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/dotcloud/docker/pkg/cgroups"
)

var (
	subsystems = map[string]subsystem{
		"devices":    &devicesGroup{},
		"memory":     &memoryGroup{},
		"cpu":        &cpuGroup{},
		"cpuset":     &cpusetGroup{},
		"cpuacct":    &cpuacctGroup{},
		"blkio":      &blkioGroup{},
		"perf_event": &perfEventGroup{},
		"freezer":    &freezerGroup{},
	}
)

type subsystem interface {
	Set(*data) error
	Remove(*data) error
	Stats(*data) (map[string]float64, error)
}

type data struct {
	root   string
	cgroup string
	c      *cgroups.Cgroup
	pid    int
}

func Apply(c *cgroups.Cgroup, pid int) (cgroups.ActiveCgroup, error) {
	// We have two implementation of cgroups support, one is based on
	// systemd and the dbus api, and one is based on raw cgroup fs operations
	// following the pre-single-writer model docs at:
	// http://www.freedesktop.org/wiki/Software/systemd/PaxControlGroups/
	//
	// we can pick any subsystem to find the root

	cgroupRoot, err := cgroups.FindCgroupMountpoint("cpu")
	if err != nil {
		return nil, err
	}
	cgroupRoot = filepath.Dir(cgroupRoot)

	if _, err := os.Stat(cgroupRoot); err != nil {
		return nil, fmt.Errorf("cgroups fs not found")
	}

	cgroup := c.Name
	if c.Parent != "" {
		cgroup = filepath.Join(c.Parent, cgroup)
	}

	d := &data{
		root:   cgroupRoot,
		cgroup: cgroup,
		c:      c,
		pid:    pid,
	}
	for _, sys := range subsystems {
		if err := sys.Set(d); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func GetStats(c *cgroups.Cgroup, subsystem string, pid int) (map[string]float64, error) {
	cgroupRoot, err := cgroups.FindCgroupMountpoint("cpu")
	if err != nil {
		return nil, err
	}
	cgroupRoot = filepath.Dir(cgroupRoot)

	if _, err := os.Stat(cgroupRoot); err != nil {
		return nil, fmt.Errorf("cgroups fs not found")
	}

	cgroup := c.Name
	if c.Parent != "" {
		cgroup = filepath.Join(c.Parent, cgroup)
	}

	d := &data{
		root:   cgroupRoot,
		cgroup: cgroup,
		c:      c,
		pid:    pid,
	}
	sys, exists := subsystems[subsystem]
	if !exists {
		return nil, fmt.Errorf("subsystem %s does not exist", subsystem)
	}
	return sys.Stats(d)
}

func (raw *data) path(subsystem string) (string, error) {
	initPath, err := cgroups.GetInitCgroupDir(subsystem)
	if err != nil {
		return "", err
	}
	return filepath.Join(raw.root, subsystem, initPath, raw.cgroup), nil
}

func (raw *data) join(subsystem string) (string, error) {
	path, err := raw.path(subsystem)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(path, 0755); err != nil && !os.IsExist(err) {
		return "", err
	}
	if err := writeFile(path, "cgroup.procs", strconv.Itoa(raw.pid)); err != nil {
		return "", err
	}
	return path, nil
}

func (raw *data) Cleanup() error {
	for _, sys := range subsystems {
		sys.Remove(raw)
	}
	return nil
}

func writeFile(dir, file, data string) error {
	return ioutil.WriteFile(filepath.Join(dir, file), []byte(data), 0700)
}

func removePath(p string, err error) error {
	if err != nil {
		return err
	}
	if p != "" {
		return os.RemoveAll(p)
	}
	return nil
}
