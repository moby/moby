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
	for _, g := range []func(*Cgroup, int) error{
		raw.setupDevices,
		raw.setupMemory,
		raw.setupCpu,
		raw.setupCpuset,
		raw.setupCpuacct,
		raw.setupBlkio,
		raw.setupPerfevent,
		raw.setupFreezer,
	} {
		if err := g(c, pid); err != nil {
			return nil, err
		}
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
	dir, err := raw.join("devices", pid)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(dir)
		}
	}()

	if !c.DeviceAccess {

		if err := writeFile(dir, "devices.deny", "a"); err != nil {
			return err
		}

		allow := []string{
			// allow mknod for any device
			"c *:* m",
			"b *:* m",

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
	dir, err := raw.join("memory", pid)
	// only return an error for memory if it was not specified
	if err != nil && (c.Memory != 0 || c.MemorySwap != 0) {
		return err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(dir)
		}
	}()

	if c.Memory != 0 || c.MemorySwap != 0 {
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
	// we don't want to join this cgroup unless it is specified
	if c.CpusetCpus != "" {
		dir, err := raw.join("cpuset", pid)
		if err != nil && c.CpusetCpus != "" {
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

func (raw *rawCgroup) setupCpuacct(c *Cgroup, pid int) error {
	// we just want to join this group even though we don't set anything
	if _, err := raw.join("cpuacct", pid); err != nil && err != ErrNotFound {
		return err
	}
	return nil
}

func (raw *rawCgroup) setupBlkio(c *Cgroup, pid int) error {
	// we just want to join this group even though we don't set anything
	if _, err := raw.join("blkio", pid); err != nil && err != ErrNotFound {
		return err
	}
	return nil
}

func (raw *rawCgroup) setupPerfevent(c *Cgroup, pid int) error {
	// we just want to join this group even though we don't set anything
	if _, err := raw.join("perf_event", pid); err != nil && err != ErrNotFound {
		return err
	}
	return nil
}

func (raw *rawCgroup) setupFreezer(c *Cgroup, pid int) error {
	// we just want to join this group even though we don't set anything
	if _, err := raw.join("freezer", pid); err != nil && err != ErrNotFound {
		return err
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
		get("cpuacct"),
		get("blkio"),
		get("perf_event"),
		get("freezer"),
	} {
		if path != "" {
			os.RemoveAll(path)
		}
	}
	return nil
}
