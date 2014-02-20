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

func ApplyCgroup(container *libcontainer.Container, pid int) (err error) {
	if container.Cgroups == nil {
		return nil
	}

	// We have two implementation of cgroups support, one is based on
	// systemd and the dbus api, and one is based on raw cgroup fs operations
	// following the pre-single-writer model docs at:
	// http://www.freedesktop.org/wiki/Software/systemd/PaxControlGroups/
	//
	// we can pick any subsystem to find the root
	cgroupRoot, err := cgroups.FindCgroupMountpoint("memory")
	if err != nil {
		return err
	}
	cgroupRoot = filepath.Dir(cgroupRoot)
	if _, err := os.Stat(cgroupRoot); err != nil {
		return fmt.Errorf("cgroups fs not found")
	}
	if err := setupDevices(container, cgroupRoot, pid); err != nil {
		return err
	}
	if err := setupMemory(container, cgroupRoot, pid); err != nil {
		return err
	}
	if err := setupCpu(container, cgroupRoot, pid); err != nil {
		return err
	}
	return nil
}

func setupDevices(container *libcontainer.Container, cgroupRoot string, pid int) (err error) {
	if !container.Cgroups.DeviceAccess {
		dir, err := container.Cgroups.Join(cgroupRoot, "devices", pid)
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

func setupMemory(container *libcontainer.Container, cgroupRoot string, pid int) (err error) {
	if container.Cgroups.Memory != 0 || container.Cgroups.MemorySwap != 0 {
		dir, err := container.Cgroups.Join(cgroupRoot, "memory", pid)
		if err != nil {
			return err
		}
		defer func() {
			if err != nil {
				os.RemoveAll(dir)
			}
		}()

		if container.Cgroups.Memory != 0 {
			if err := writeFile(dir, "memory.limit_in_bytes", strconv.FormatInt(container.Cgroups.Memory, 10)); err != nil {
				return err
			}
			if err := writeFile(dir, "memory.soft_limit_in_bytes", strconv.FormatInt(container.Cgroups.Memory, 10)); err != nil {
				return err
			}
		}
		if container.Cgroups.MemorySwap != 0 {
			if err := writeFile(dir, "memory.memsw.limit_in_bytes", strconv.FormatInt(container.Cgroups.MemorySwap, 10)); err != nil {
				return err
			}
		}
	}
	return nil
}

func setupCpu(container *libcontainer.Container, cgroupRoot string, pid int) (err error) {
	// We always want to join the cpu group, to allow fair cpu scheduling
	// on a container basis
	dir, err := container.Cgroups.Join(cgroupRoot, "cpu", pid)
	if err != nil {
		return err
	}
	if container.Cgroups.CpuShares != 0 {
		if err := writeFile(dir, "cpu.shares", strconv.FormatInt(container.Cgroups.CpuShares, 10)); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(dir, file, data string) error {
	return ioutil.WriteFile(filepath.Join(dir, file), []byte(data), 0700)
}
