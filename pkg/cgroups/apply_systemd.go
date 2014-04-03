// +build linux

package cgroups

import (
	systemd1 "github.com/coreos/go-systemd/dbus"
	"github.com/dotcloud/docker/pkg/systemd"
	"github.com/godbus/dbus"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type systemdCgroup struct {
	cleanupDirs []string
}

var (
	connLock              sync.Mutex
	theConn               *systemd1.Conn
	hasStartTransientUnit bool
)

func useSystemd() bool {
	if !systemd.SdBooted() {
		return false
	}

	connLock.Lock()
	defer connLock.Unlock()

	if theConn == nil {
		var err error
		theConn, err = systemd1.New()
		if err != nil {
			return false
		}

		// Assume we have StartTransientUnit
		hasStartTransientUnit = true

		// But if we get UnknownMethod error we don't
		if _, err := theConn.StartTransientUnit("test.scope", "invalid"); err != nil {
			if dbusError, ok := err.(dbus.Error); ok {
				if dbusError.Name == "org.freedesktop.DBus.Error.UnknownMethod" {
					hasStartTransientUnit = false
				}
			}
		}
	}

	return hasStartTransientUnit
}

type DeviceAllow struct {
	Node        string
	Permissions string
}

func getIfaceForUnit(unitName string) string {
	if strings.HasSuffix(unitName, ".scope") {
		return "Scope"
	}
	if strings.HasSuffix(unitName, ".service") {
		return "Service"
	}
	return "Unit"
}

type KeyValue struct {
	Key   string
	Value string
}

func systemdApply(c *Cgroup, pid int) (ActiveCgroup, error) {
	unitName := c.Parent + "-" + c.Name + ".scope"
	slice := "system.slice"

	var properties []systemd1.Property

	var (
		cpuArgs    []KeyValue
		cpusetArgs []KeyValue
		memoryArgs []KeyValue
		res        systemdCgroup
	)

	// First set up things not supported by systemd

	// -1 disables memorySwap
	if c.MemorySwap >= 0 && (c.Memory != 0 || c.MemorySwap > 0) {
		memorySwap := c.MemorySwap

		if memorySwap == 0 {
			// By default, MemorySwap is set to twice the size of RAM.
			memorySwap = c.Memory * 2
		}

		memoryArgs = append(memoryArgs, KeyValue{"memory.memsw.limit_in_bytes", strconv.FormatInt(memorySwap, 10)})
	}

	if c.CpuQuota != 0 {
		cpuArgs = append(cpuArgs, KeyValue{"cpu.cfs_quota_us", strconv.FormatInt(c.CpuQuota, 10)})
	}

	if c.CpusetCpus != "" {
		cpusetArgs = append(cpusetArgs, KeyValue{"cpuset.cpus", c.CpusetCpus})
	}

	if c.CpusetMems != "" {
		cpusetArgs = append(cpusetArgs, KeyValue{"cpuset.mems", c.CpusetMems})
	}

	if c.Slice != "" {
		slice = c.Slice
	}

	properties = append(properties,
		systemd1.Property{"Slice", dbus.MakeVariant(slice)},
		systemd1.Property{"Description", dbus.MakeVariant("docker container " + c.Name)},
		systemd1.Property{"PIDs", dbus.MakeVariant([]uint32{uint32(pid)})})

	if !c.DeviceAccess {
		properties = append(properties,
			systemd1.Property{"DevicePolicy", dbus.MakeVariant("strict")},
			systemd1.Property{"DeviceAllow", dbus.MakeVariant([]DeviceAllow{
				{"/dev/null", "rwm"},
				{"/dev/zero", "rwm"},
				{"/dev/full", "rwm"},
				{"/dev/random", "rwm"},
				{"/dev/urandom", "rwm"},
				{"/dev/tty", "rwm"},
				{"/dev/console", "rwm"},
				{"/dev/tty0", "rwm"},
				{"/dev/tty1", "rwm"},
				{"/dev/pts/ptmx", "rwm"},
				// There is no way to add /dev/pts/* here atm, so we hack this manually below
				// /dev/pts/* (how to add this?)
				// Same with tuntap, which doesn't exist as a node most of the time
			})})
	}

	// Ensure we have a memory cgroup if we have any memory args
	if c.MemoryAccounting || len(memoryArgs) != 0 {
		properties = append(properties,
			systemd1.Property{"MemoryAccounting", dbus.MakeVariant(true)})
	}

	// Ensure we have a cpu cgroup if we have any cpu args
	if c.CpuAccounting || len(cpuArgs) != 0 {
		properties = append(properties,
			systemd1.Property{"CPUAccounting", dbus.MakeVariant(true)})
	}

	if c.Memory != 0 {
		properties = append(properties,
			systemd1.Property{"MemoryLimit", dbus.MakeVariant(uint64(c.Memory))})
	}

	if c.CpuShares != 0 {
		properties = append(properties,
			systemd1.Property{"CPUShares", dbus.MakeVariant(uint64(c.CpuShares))})
	}

	if _, err := theConn.StartTransientUnit(unitName, "replace", properties...); err != nil {
		return nil, err
	}

	// To work around the lack of /dev/pts/* support above we need to manually add these
	// so, ask systemd for the cgroup used
	props, err := theConn.GetUnitTypeProperties(unitName, getIfaceForUnit(unitName))
	if err != nil {
		return nil, err
	}

	cgroup := props["ControlGroup"].(string)

	if !c.DeviceAccess {
		mountpoint, err := FindCgroupMountpoint("devices")
		if err != nil {
			return nil, err
		}

		path := filepath.Join(mountpoint, cgroup)

		// /dev/pts/*
		if err := writeFile(path, "devices.allow", "c 136:* rwm"); err != nil {
			return nil, err
		}
		// tuntap
		if err := writeFile(path, "devices.allow", "c 10:200 rwm"); err != nil {
			return nil, err
		}
	}

	if len(cpuArgs) != 0 {
		mountpoint, err := FindCgroupMountpoint("cpu")
		if err != nil {
			return nil, err
		}

		path := filepath.Join(mountpoint, cgroup)

		for _, kv := range cpuArgs {
			if err := writeFile(path, kv.Key, kv.Value); err != nil {
				return nil, err
			}
		}
	}

	if len(memoryArgs) != 0 {
		mountpoint, err := FindCgroupMountpoint("memory")
		if err != nil {
			return nil, err
		}

		path := filepath.Join(mountpoint, cgroup)

		for _, kv := range memoryArgs {
			if err := writeFile(path, kv.Key, kv.Value); err != nil {
				return nil, err
			}
		}
	}

	if len(cpusetArgs) != 0 {
		// systemd does not atm set up the cpuset controller, so we must manually
		// join it. Additionally that is a very finicky controller where each
		// level must have a full setup as the default for a new directory is "no cpus",
		// so we avoid using any hierarchies here, creating a toplevel directory.
		mountpoint, err := FindCgroupMountpoint("cpuset")
		if err != nil {
			return nil, err
		}
		initPath, err := GetInitCgroupDir("cpuset")
		if err != nil {
			return nil, err
		}

		rootPath := filepath.Join(mountpoint, initPath)

		path := filepath.Join(mountpoint, initPath, c.Parent+"-"+c.Name)

		res.cleanupDirs = append(res.cleanupDirs, path)

		if err := os.MkdirAll(path, 0755); err != nil && !os.IsExist(err) {
			return nil, err
		}

		foundCpus := false
		foundMems := false

		for _, kv := range cpusetArgs {
			if kv.Key == "cpuset.cpus" {
				foundCpus = true
			}
			if kv.Key == "cpuset.mems" {
				foundMems = true
			}
			if err := writeFile(path, kv.Key, kv.Value); err != nil {
				return nil, err
			}
		}

		// These are required, if not specified inherit from parent
		if !foundCpus {
			s, err := ioutil.ReadFile(filepath.Join(rootPath, "cpuset.cpus"))
			if err != nil {
				return nil, err
			}

			if err := writeFile(path, "cpuset.cpus", string(s)); err != nil {
				return nil, err
			}
		}

		// These are required, if not specified inherit from parent
		if !foundMems {
			s, err := ioutil.ReadFile(filepath.Join(rootPath, "cpuset.mems"))
			if err != nil {
				return nil, err
			}

			if err := writeFile(path, "cpuset.mems", string(s)); err != nil {
				return nil, err
			}
		}

		if err := writeFile(path, "cgroup.procs", strconv.Itoa(pid)); err != nil {
			return nil, err
		}
	}

	return &res, nil
}

func (c *systemdCgroup) Cleanup() error {
	// systemd cleans up, we don't need to do much

	for _, path := range c.cleanupDirs {
		os.RemoveAll(path)
	}

	return nil
}
