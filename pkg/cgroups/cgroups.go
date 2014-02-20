package cgroups

import (
	"bufio"
	"fmt"
	"github.com/dotcloud/docker/pkg/mount"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Cgroup struct {
	Name   string `json:"name,omitempty"`
	Parent string `json:"parent,omitempty"`

	DeviceAccess bool  `json:"device_access,omitempty"` // name of parent cgroup or slice
	Memory       int64 `json:"memory,omitempty"`        // Memory limit (in bytes)
	MemorySwap   int64 `json:"memory_swap,omitempty"`   // Total memory usage (memory + swap); set `-1' to disable swap
	CpuShares    int64 `json:"cpu_shares,omitempty"`    // CPU shares (relative weight vs. other containers)
}

// https://www.kernel.org/doc/Documentation/cgroups/cgroups.txt
func FindCgroupMountpoint(subsystem string) (string, error) {
	mounts, err := mount.GetMounts()
	if err != nil {
		return "", err
	}

	for _, mount := range mounts {
		if mount.Fstype == "cgroup" {
			for _, opt := range strings.Split(mount.VfsOpts, ",") {
				if opt == subsystem {
					return mount.Mountpoint, nil
				}
			}
		}
	}
	return "", fmt.Errorf("cgroup mountpoint not found for %s", subsystem)
}

// Returns the relative path to the cgroup docker is running in.
func GetThisCgroupDir(subsystem string) (string, error) {
	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	defer f.Close()

	return parseCgroupFile(subsystem, f)
}

func GetInitCgroupDir(subsystem string) (string, error) {
	f, err := os.Open("/proc/1/cgroup")
	if err != nil {
		return "", err
	}
	defer f.Close()

	return parseCgroupFile(subsystem, f)
}

func (c *Cgroup) Path(root, subsystem string) (string, error) {
	cgroup := c.Name
	if c.Parent != "" {
		cgroup = filepath.Join(c.Parent, cgroup)
	}
	initPath, err := GetInitCgroupDir(subsystem)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, subsystem, initPath, cgroup), nil
}

func (c *Cgroup) Join(root, subsystem string, pid int) (string, error) {
	path, err := c.Path(root, subsystem)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(path, 0755); err != nil && !os.IsExist(err) {
		return "", err
	}
	if err := writeFile(path, "tasks", strconv.Itoa(pid)); err != nil {
		return "", err
	}
	return path, nil
}

func (c *Cgroup) Cleanup(root string) error {
	get := func(subsystem string) string {
		path, _ := c.Path(root, subsystem)
		return path
	}

	for _, path := range []string{
		get("memory"),
		get("devices"),
		get("cpu"),
	} {
		os.RemoveAll(path)
	}
	return nil
}

func parseCgroupFile(subsystem string, r io.Reader) (string, error) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		if err := s.Err(); err != nil {
			return "", err
		}
		text := s.Text()
		parts := strings.Split(text, ":")
		for _, subs := range strings.Split(parts[1], ",") {
			if subs == subsystem {
				return parts[2], nil
			}
		}
	}
	return "", fmt.Errorf("cgroup '%s' not found in /proc/self/cgroup", subsystem)
}

func writeFile(dir, file, data string) error {
	return ioutil.WriteFile(filepath.Join(dir, file), []byte(data), 0700)
}
