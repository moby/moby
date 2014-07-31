package fs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/docker/libcontainer/cgroups"
)

var (
	subsystems = map[string]subsystem{
		"devices":    &DevicesGroup{},
		"memory":     &MemoryGroup{},
		"cpu":        &CpuGroup{},
		"cpuset":     &CpusetGroup{},
		"cpuacct":    &CpuacctGroup{},
		"blkio":      &BlkioGroup{},
		"perf_event": &PerfEventGroup{},
		"freezer":    &FreezerGroup{},
	}
)

type subsystem interface {
	Set(*data) error
	Remove(*data) error
	GetStats(string, *cgroups.Stats) error
}

type data struct {
	root   string
	cgroup string
	c      *cgroups.Cgroup
	pid    int
}

func Apply(c *cgroups.Cgroup, pid int) (cgroups.ActiveCgroup, error) {
	d, err := getCgroupData(c, pid)
	if err != nil {
		return nil, err
	}

	for _, sys := range subsystems {
		if err := sys.Set(d); err != nil {
			d.Cleanup()
			return nil, err
		}
	}

	return d, nil
}

func Cleanup(c *cgroups.Cgroup) error {
	d, err := getCgroupData(c, 0)
	if err != nil {
		return fmt.Errorf("Could not get Cgroup data %s", err)
	}
	return d.Cleanup()
}

func GetStats(c *cgroups.Cgroup) (*cgroups.Stats, error) {
	stats := cgroups.NewStats()

	d, err := getCgroupData(c, 0)
	if err != nil {
		return nil, fmt.Errorf("getting CgroupData %s", err)
	}

	for sysname, sys := range subsystems {
		path, err := d.path(sysname)
		if err != nil {
			// Don't fail if a cgroup hierarchy was not found, just skip this subsystem
			if err == cgroups.ErrNotFound {
				continue
			}

			return nil, err
		}

		if err := sys.GetStats(path, stats); err != nil {
			return nil, err
		}
	}

	return stats, nil
}

// Freeze toggles the container's freezer cgroup depending on the state
// provided
func Freeze(c *cgroups.Cgroup, state cgroups.FreezerState) error {
	d, err := getCgroupData(c, 0)
	if err != nil {
		return err
	}

	c.Freezer = state

	freezer := subsystems["freezer"]

	return freezer.Set(d)
}

func GetPids(c *cgroups.Cgroup) ([]int, error) {
	d, err := getCgroupData(c, 0)
	if err != nil {
		return nil, err
	}

	dir, err := d.path("devices")
	if err != nil {
		return nil, err
	}

	return cgroups.ReadProcsFile(dir)
}

func getCgroupData(c *cgroups.Cgroup, pid int) (*data, error) {
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

	return &data{
		root:   cgroupRoot,
		cgroup: cgroup,
		c:      c,
		pid:    pid,
	}, nil
}

func (raw *data) parent(subsystem string) (string, error) {
	initPath, err := cgroups.GetInitCgroupDir(subsystem)
	if err != nil {
		return "", err
	}
	return filepath.Join(raw.root, subsystem, initPath), nil
}

func (raw *data) path(subsystem string) (string, error) {
	// If the cgroup name/path is absolute do not look relative to the cgroup of the init process.
	if filepath.IsAbs(raw.cgroup) {
		return filepath.Join(raw.root, subsystem, raw.cgroup), nil
	}
	parent, err := raw.parent(subsystem)
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, raw.cgroup), nil
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

func readFile(dir, file string) (string, error) {
	data, err := ioutil.ReadFile(filepath.Join(dir, file))
	return string(data), err
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
