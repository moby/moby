package cgroups

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

type rawCgroup struct {
	root       string
	cgroup     string
	cgroupFlat string
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
	cgroupFlat := c.Name

	if c.Parent != "" {
		cgroup = filepath.Join(c.Parent, cgroup)
		cgroupFlat = c.Parent + "-" + cgroupFlat
	}

	raw := &rawCgroup{
		root:       cgroupRoot,
		cgroup:     cgroup,
		cgroupFlat: cgroupFlat,
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

func (raw *rawCgroup) path(subsystem string, flat bool) (string, error) {
	initPath, err := GetInitCgroupDir(subsystem)
	if err != nil {
		return "", err
	}
	cgroup := raw.cgroup
	if flat {
		cgroup = raw.cgroupFlat
	}

	return filepath.Join(raw.root, subsystem, initPath, cgroup), nil
}

func (raw *rawCgroup) ensure(subsystem string, flat bool) (string, error) {
	path, err := raw.path(subsystem, flat)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(path, 0755); err != nil && !os.IsExist(err) {
		return "", err
	}
	return path, nil
}

func (raw *rawCgroup) join(subsystem string, flat bool, pid int) (string, error) {
	path, err := raw.ensure(subsystem, flat)
	if err != nil {
		return "", err
	}
	if err := writeFile(path, "cgroup.procs", strconv.Itoa(pid)); err != nil {
		return "", err
	}
	return path, nil
}

func (raw *rawCgroup) setupDevices(c *Cgroup, pid int) (err error) {
	if !c.DeviceAccess {
		dir, e0 := raw.join("devices", false, pid)
		if e0 != nil {
			return e0
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
	if c.MemoryAccounting || c.Memory != 0 || c.MemorySwap != 0 {
		dir, e0 := raw.join("memory", false, pid)
		if e0 != nil {
			return e0
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
		if c.MemorySwap == 0 && c.Memory != 0 {
			if err := writeFile(dir, "memory.memsw.limit_in_bytes", strconv.FormatInt(c.Memory*2, 10)); err != nil {
				return err
			}
		} else if c.MemorySwap > 0 {
			if err := writeFile(dir, "memory.memsw.limit_in_bytes", strconv.FormatInt(c.MemorySwap, 10)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (raw *rawCgroup) setupCpu(c *Cgroup, pid int) (err error) {
	// We always want to join the cpu group, to allow fair cpu scheduling
	// on a container basis
	dir, e0 := raw.join("cpu", false, pid)
	if e0 != nil {
		return e0
	}
	defer func() {
		if err != nil {
			os.RemoveAll(dir)
		}
	}()
	if c.CpuShares != 0 {
		if err := writeFile(dir, "cpu.shares", strconv.FormatInt(c.CpuShares, 10)); err != nil {
			return err
		}
	}
	if c.CpuQuota != 0 {
		if err := writeFile(dir, "cpu.cfs_quota_us", strconv.FormatInt(c.CpuQuota, 10)); err != nil {
			return err
		}
	}
	return nil
}

func (raw *rawCgroup) setupCpuset(c *Cgroup, pid int) (err error) {
	if c.CpusetCpus != "" || c.CpusetMems != "" {
		// The cpuset controller is very finicky where each level must have
		// a full setup as the default for a new directory is "no cpus", so
		// we avoid using any hierarchies here, creating a toplevel directory,
		// so we pass flat == true
		dir, e0 := raw.ensure("cpuset", true)
		if e0 != nil {
			return e0
		}
		defer func() {
			if err != nil {
				os.RemoveAll(dir)
			}
		}()

		if c.CpusetCpus != "" {
			if err := writeFile(dir, "cpuset.cpus", c.CpusetCpus); err != nil {
				return err
			}
		} else {
			s, err := ioutil.ReadFile(filepath.Join(filepath.Dir(dir), "cpuset.cpus"))
			if err != nil {
				return err
			}

			if err := writeFile(dir, "cpuset.cpus", string(s)); err != nil {
				return err
			}
		}

		if c.CpusetMems != "" {
			if err := writeFile(dir, "cpuset.mems", c.CpusetMems); err != nil {
				return err
			}
		} else {
			s, err := ioutil.ReadFile(filepath.Join(filepath.Dir(dir), "cpuset.mems"))
			if err != nil {
				return err
			}

			if err := writeFile(dir, "cpuset.mems", string(s)); err != nil {
				return err
			}
		}

		_, err = raw.join("cpuset", true, pid)
		if err != nil {
			return err
		}
	}
	return nil
}

func (raw *rawCgroup) Cleanup() error {
	get := func(subsystem string) string {
		path, _ := raw.path(subsystem, subsystem == "cpuset")
		return path
	}

	for _, path := range []string{
		get("memory"),
		get("devices"),
		get("cpu"),
		get("cpuset"),
		get("blkio"),
	} {
		if path != "" {
			os.RemoveAll(path)
		}
	}
	return nil
}
