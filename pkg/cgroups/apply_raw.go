package cgroups

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type rawCgroup struct {
	root   string
	cgroup string
}

func rawApply(c *Cgroup, pid int) (ActiveCgroup, error) {
	// We have two implementation of cgroups support, one is based on
	// systemd and the dbus api, and one is based on raw cgroup fs operations
	// following the pre-single-writer model docs at:
	// http://www.freedesktop.org/wiki/Software/systemd/PaxControlGroups/
	//
	// we can pick any subsystem to find the root

	cgroupRoot, err := FindCgroupMountpoint("cpu")
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

	raw := &rawCgroup{
		root:   cgroupRoot,
		cgroup: cgroup,
	}

	if err := raw.setupDevices(c, pid); err != nil {
		return nil, err
	}
	if err := raw.setupMemory(c, pid); err != nil {
		return nil, err
	}
	if err := raw.setupCpu(c, pid); err != nil {
		return nil, err
	}
	if err := raw.setupCpuset(c, pid); err != nil {
		return nil, err
	}
	return raw, nil
}

func (raw *rawCgroup) path(subsystem string) (string, error) {
	initPath, err := GetInitCgroupDir(subsystem)
	if err != nil {
		return "", err
	}
	return filepath.Join(raw.root, subsystem, initPath, raw.cgroup), nil
}

func (raw *rawCgroup) join(subsystem string, pid int) (string, error) {
	path, err := raw.path(subsystem)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(path, 0755); err != nil && !os.IsExist(err) {
		return "", err
	}
	if err := writeFile(path, "cgroup.procs", strconv.Itoa(pid)); err != nil {
		return "", err
	}
	return path, nil
}

func (raw *rawCgroup) setupDevices(c *Cgroup, pid int) (err error) {
	if !c.DeviceAccess {
		dir, err := raw.join("devices", pid)
		if err != nil {
			return err
		}

		defer func() {
			if err != nil {
				os.RemoveAll(dir)
			}
		}()

		if err := writeFile(dir, "devices.deny", "a"); err != nil {
			return err
		}

		allow := []string{
			// /dev/null, zero, full
			"c 1:3 rwm",
			"c 1:5 rwm",
			"c 1:7 rwm",

			// consoles
			"c 5:1 rwm",
			"c 5:0 rwm",
			"c 4:0 rwm",
			"c 4:1 rwm",

			// /dev/urandom,/dev/random
			"c 1:9 rwm",
			"c 1:8 rwm",

			// /dev/pts/ - pts namespaces are "coming soon"
			"c 136:* rwm",
			"c 5:2 rwm",

			// tuntap
			"c 10:200 rwm",
		}

		for _, val := range allow {
			if err := writeFile(dir, "devices.allow", val); err != nil {
				return err
			}
		}
	}
	return nil
}

func (raw *rawCgroup) setupMemory(c *Cgroup, pid int) (err error) {
	if c.Memory != 0 || c.MemorySwap != 0 {
		dir, err := raw.join("memory", pid)
		if err != nil {
			return err
		}
		defer func() {
			if err != nil {
				os.RemoveAll(dir)
			}
		}()

		if c.Memory != 0 {
			if err := writeFile(dir, "memory.limit_in_bytes", strconv.FormatInt(c.Memory, 10)); err != nil {
				return err
			}
			if err := writeFile(dir, "memory.soft_limit_in_bytes", strconv.FormatInt(c.Memory, 10)); err != nil {
				return err
			}
		}
		// By default, MemorySwap is set to twice the size of RAM.
		// If you want to omit MemorySwap, set it to `-1'.
		if c.MemorySwap != -1 {
			if err := writeFile(dir, "memory.memsw.limit_in_bytes", strconv.FormatInt(c.Memory*2, 10)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (raw *rawCgroup) setupCpu(c *Cgroup, pid int) (err error) {
	// We always want to join the cpu group, to allow fair cpu scheduling
	// on a container basis
	dir, err := raw.join("cpu", pid)
	if err != nil {
		return err
	}
	if c.CpuShares != 0 {
		if err := writeFile(dir, "cpu.shares", strconv.FormatInt(c.CpuShares, 10)); err != nil {
			return err
		}
	}
	return nil
}

func (raw *rawCgroup) setupCpuset(c *Cgroup, pid int) (err error) {
	if c.CpusetCpus != "" {
		dir, err := raw.join("cpuset", pid)
		if err != nil {
			return err
		}
		defer func() {
			if err != nil {
				os.RemoveAll(dir)
			}
		}()

		if err := writeFile(dir, "cpuset.cpus", c.CpusetCpus); err != nil {
			return err
		}
	}
	return nil
}

func (raw *rawCgroup) Cleanup() error {
	get := func(subsystem string) string {
		path, _ := raw.path(subsystem)
		return path
	}

	for _, path := range []string{
		get("memory"),
		get("devices"),
		get("cpu"),
		get("cpuset"),
	} {
		if path != "" {
			os.RemoveAll(path)
		}
	}
	return nil
}
