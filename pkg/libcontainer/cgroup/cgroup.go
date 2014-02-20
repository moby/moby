package cgroup

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

// We have two implementation of cgroups support, one is based on
// systemd and the dbus api, and one is based on raw cgroup fs operations
// following the pre-single-writer model docs at:
// http://www.freedesktop.org/wiki/Software/systemd/PaxControlGroups/
const (
	cgroupRoot = "/sys/fs/cgroup"
)

func useSystemd() bool {
	return false
}

func applyCgroupSystemd(container *libcontainer.Container, pid int) error {
	return fmt.Errorf("not supported yet")
}

func writeFile(dir, file, data string) error {
	return ioutil.WriteFile(filepath.Join(dir, file), []byte(data), 0700)
}

func getCgroup(subsystem string, container *libcontainer.Container) (string, error) {
	cgroup := container.CgroupName
	if container.CgroupParent != "" {
		cgroup = filepath.Join(container.CgroupParent, cgroup)
	}

	initPath, err := cgroups.GetInitCgroupDir(subsystem)
	if err != nil {
		return "", err
	}

	path := filepath.Join(cgroupRoot, subsystem, initPath, cgroup)

	return path, nil
}

func joinCgroup(subsystem string, container *libcontainer.Container, pid int) (string, error) {
	path, err := getCgroup(subsystem, container)
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

func applyCgroupRaw(container *libcontainer.Container, pid int) (retErr error) {
	if _, err := os.Stat(cgroupRoot); err != nil {
		return fmt.Errorf("cgroups fs not found")
	}

	if !container.DeviceAccess {
		dir, err := joinCgroup("devices", container, pid)
		if err != nil {
			return err
		}
		defer func() {
			if retErr != nil {
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

	if container.Memory != 0 || container.MemorySwap != 0 {
		dir, err := joinCgroup("memory", container, pid)
		if err != nil {
			return err
		}
		defer func() {
			if retErr != nil {
				os.RemoveAll(dir)
			}
		}()

		if container.Memory != 0 {
			if err := writeFile(dir, "memory.limit_in_bytes", strconv.FormatInt(container.Memory, 10)); err != nil {
				return err
			}
			if err := writeFile(dir, "memory.soft_limit_in_bytes", strconv.FormatInt(container.Memory, 10)); err != nil {
				return err
			}
		}
		if container.MemorySwap != 0 {
			if err := writeFile(dir, "memory.memsw.limit_in_bytes", strconv.FormatInt(container.MemorySwap, 10)); err != nil {
				return err
			}
		}
	}

	// We always want to join the cpu group, to allow fair cpu scheduling
	// on a container basis
	dir, err := joinCgroup("cpu", container, pid)
	if err != nil {
		return err
	}
	if container.CpuShares != 0 {
		if err := writeFile(dir, "cpu.shares", strconv.FormatInt(container.CpuShares, 10)); err != nil {
			return err
		}
	}
	return nil
}

func CleanupCgroup(container *libcontainer.Container) error {
	path, _ := getCgroup("memory", container)
	os.RemoveAll(path)
	path, _ = getCgroup("devices", container)
	os.RemoveAll(path)
	path, _ = getCgroup("cpu", container)
	os.RemoveAll(path)
	return nil
}

func ApplyCgroup(container *libcontainer.Container, pid int) error {
	if container.CgroupName == "" {
		return nil
	}

	if useSystemd() {
		return applyCgroupSystemd(container, pid)
	} else {
		return applyCgroupRaw(container, pid)
	}
}
