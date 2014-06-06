// +build linux

package systemd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	systemd1 "github.com/coreos/go-systemd/dbus"
	"github.com/dotcloud/docker/pkg/libcontainer/cgroups"
	"github.com/dotcloud/docker/pkg/systemd"
	"github.com/godbus/dbus"
)

type systemdCgroup struct {
	cleanupDirs []string
}

var (
	connLock              sync.Mutex
	theConn               *systemd1.Conn
	hasStartTransientUnit bool
)

func UseSystemd() bool {
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

func getIfaceForUnit(unitName string) string {
	if strings.HasSuffix(unitName, ".scope") {
		return "Scope"
	}
	if strings.HasSuffix(unitName, ".service") {
		return "Service"
	}
	return "Unit"
}

type cgroupArg struct {
	File  string
	Value string
}

func Apply(c *cgroups.Cgroup, pid int) (cgroups.ActiveCgroup, error) {
	var (
		unitName   = getUnitName(c)
		slice      = "system.slice"
		properties []systemd1.Property
		cpuArgs    []cgroupArg
		cpusetArgs []cgroupArg
		memoryArgs []cgroupArg
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

		memoryArgs = append(memoryArgs, cgroupArg{"memory.memsw.limit_in_bytes", strconv.FormatInt(memorySwap, 10)})
	}

	if c.CpusetCpus != "" {
		cpusetArgs = append(cpusetArgs, cgroupArg{"cpuset.cpus", c.CpusetCpus})
	}

	if c.Slice != "" {
		slice = c.Slice
	}

	properties = append(properties,
		systemd1.Property{"Slice", dbus.MakeVariant(slice)},
		systemd1.Property{"Description", dbus.MakeVariant("docker container " + c.Name)},
		systemd1.Property{"PIDs", dbus.MakeVariant([]uint32{uint32(pid)})},
	)

	// Always enable accounting, this gets us the same behaviour as the fs implementation,
	// plus the kernel has some problems with joining the memory cgroup at a later time.
	properties = append(properties,
		systemd1.Property{"MemoryAccounting", dbus.MakeVariant(true)},
		systemd1.Property{"CPUAccounting", dbus.MakeVariant(true)},
		systemd1.Property{"BlockIOAccounting", dbus.MakeVariant(true)})

	if c.Memory != 0 {
		properties = append(properties,
			systemd1.Property{"MemoryLimit", dbus.MakeVariant(uint64(c.Memory))})
	}
	// TODO: MemoryReservation and MemorySwap not available in systemd

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

	if !c.AllowAllDevices {
		// Atm we can't use the systemd device support because of two missing things:
		// * Support for wildcards to allow mknod on any device
		// * Support for wildcards to allow /dev/pts support
		//
		// The second is available in more recent systemd as "char-pts", but not in e.g. v208 which is
		// in wide use. When both these are availalable we will be able to switch, but need to keep the old
		// implementation for backwards compat.
		//
		// Note: we can't use systemd to set up the initial limits, and then change the cgroup
		// because systemd will re-write the device settings if it needs to re-apply the cgroup context.
		// This happens at least for v208 when any sibling unit is started.

		mountpoint, err := cgroups.FindCgroupMountpoint("devices")
		if err != nil {
			return nil, err
		}

		initPath, err := cgroups.GetInitCgroupDir("devices")
		if err != nil {
			return nil, err
		}

		dir := filepath.Join(mountpoint, initPath, c.Parent, c.Name)

		res.cleanupDirs = append(res.cleanupDirs, dir)

		if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
			return nil, err
		}

		if err := ioutil.WriteFile(filepath.Join(dir, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0700); err != nil {
			return nil, err
		}

		if err := writeFile(dir, "devices.deny", "a"); err != nil {
			return nil, err
		}

		for _, dev := range c.AllowedDevices {
			if err := writeFile(dir, "devices.allow", dev.GetCgroupAllowString()); err != nil {
				return nil, err
			}
		}
	}

	if len(cpuArgs) != 0 {
		mountpoint, err := cgroups.FindCgroupMountpoint("cpu")
		if err != nil {
			return nil, err
		}

		path := filepath.Join(mountpoint, cgroup)

		for _, arg := range cpuArgs {
			if err := ioutil.WriteFile(filepath.Join(path, arg.File), []byte(arg.Value), 0700); err != nil {
				return nil, err
			}
		}
	}

	if len(memoryArgs) != 0 {
		mountpoint, err := cgroups.FindCgroupMountpoint("memory")
		if err != nil {
			return nil, err
		}

		path := filepath.Join(mountpoint, cgroup)

		for _, arg := range memoryArgs {
			if err := ioutil.WriteFile(filepath.Join(path, arg.File), []byte(arg.Value), 0700); err != nil {
				return nil, err
			}
		}
	}

	// we need to manually join the freezer cgroup in systemd because it does not currently support it
	// via the dbus api
	freezerPath, err := joinFreezer(c, pid)
	if err != nil {
		return nil, err
	}
	res.cleanupDirs = append(res.cleanupDirs, freezerPath)

	if len(cpusetArgs) != 0 {
		// systemd does not atm set up the cpuset controller, so we must manually
		// join it. Additionally that is a very finicky controller where each
		// level must have a full setup as the default for a new directory is "no cpus",
		// so we avoid using any hierarchies here, creating a toplevel directory.
		mountpoint, err := cgroups.FindCgroupMountpoint("cpuset")
		if err != nil {
			return nil, err
		}

		initPath, err := cgroups.GetInitCgroupDir("cpuset")
		if err != nil {
			return nil, err
		}

		var (
			foundCpus bool
			foundMems bool

			rootPath = filepath.Join(mountpoint, initPath)
			path     = filepath.Join(mountpoint, initPath, c.Parent+"-"+c.Name)
		)

		res.cleanupDirs = append(res.cleanupDirs, path)

		if err := os.MkdirAll(path, 0755); err != nil && !os.IsExist(err) {
			return nil, err
		}

		for _, arg := range cpusetArgs {
			if arg.File == "cpuset.cpus" {
				foundCpus = true
			}
			if arg.File == "cpuset.mems" {
				foundMems = true
			}
			if err := ioutil.WriteFile(filepath.Join(path, arg.File), []byte(arg.Value), 0700); err != nil {
				return nil, err
			}
		}

		// These are required, if not specified inherit from parent
		if !foundCpus {
			s, err := ioutil.ReadFile(filepath.Join(rootPath, "cpuset.cpus"))
			if err != nil {
				return nil, err
			}

			if err := ioutil.WriteFile(filepath.Join(path, "cpuset.cpus"), s, 0700); err != nil {
				return nil, err
			}
		}

		// These are required, if not specified inherit from parent
		if !foundMems {
			s, err := ioutil.ReadFile(filepath.Join(rootPath, "cpuset.mems"))
			if err != nil {
				return nil, err
			}

			if err := ioutil.WriteFile(filepath.Join(path, "cpuset.mems"), s, 0700); err != nil {
				return nil, err
			}
		}

		if err := ioutil.WriteFile(filepath.Join(path, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0700); err != nil {
			return nil, err
		}
	}

	return &res, nil
}

func writeFile(dir, file, data string) error {
	return ioutil.WriteFile(filepath.Join(dir, file), []byte(data), 0700)
}

func (c *systemdCgroup) Cleanup() error {
	// systemd cleans up, we don't need to do much

	for _, path := range c.cleanupDirs {
		os.RemoveAll(path)
	}

	return nil
}

func joinFreezer(c *cgroups.Cgroup, pid int) (string, error) {
	path, err := getFreezerPath(c)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(path, 0755); err != nil && !os.IsExist(err) {
		return "", err
	}

	if err := ioutil.WriteFile(filepath.Join(path, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0700); err != nil {
		return "", err
	}

	return path, nil
}

func getFreezerPath(c *cgroups.Cgroup) (string, error) {
	mountpoint, err := cgroups.FindCgroupMountpoint("freezer")
	if err != nil {
		return "", err
	}

	initPath, err := cgroups.GetInitCgroupDir("freezer")
	if err != nil {
		return "", err
	}

	return filepath.Join(mountpoint, initPath, fmt.Sprintf("%s-%s", c.Parent, c.Name)), nil

}

func Freeze(c *cgroups.Cgroup, state cgroups.FreezerState) error {
	path, err := getFreezerPath(c)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(path, "freezer.state"), []byte(state), 0); err != nil {
		return err
	}
	for {
		state_, err := ioutil.ReadFile(filepath.Join(path, "freezer.state"))
		if err != nil {
			return err
		}
		if string(state) == string(bytes.TrimSpace(state_)) {
			break
		}
		time.Sleep(1 * time.Millisecond)
	}
	return nil
}

func GetPids(c *cgroups.Cgroup) ([]int, error) {
	unitName := getUnitName(c)

	mountpoint, err := cgroups.FindCgroupMountpoint("cpu")
	if err != nil {
		return nil, err
	}

	props, err := theConn.GetUnitTypeProperties(unitName, getIfaceForUnit(unitName))
	if err != nil {
		return nil, err
	}
	cgroup := props["ControlGroup"].(string)

	return cgroups.ReadProcsFile(filepath.Join(mountpoint, cgroup))
}

func getUnitName(c *cgroups.Cgroup) string {
	return fmt.Sprintf("%s-%s.scope", c.Parent, c.Name)
}
