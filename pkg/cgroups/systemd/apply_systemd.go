// +build linux

package systemd

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"

	systemd1 "github.com/coreos/go-systemd/dbus"
	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/systemd"
	"github.com/godbus/dbus"
)

type systemdCgroup struct {
}

type DeviceAllow struct {
	Node        string
	Permissions string
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

func Apply(c *cgroups.Cgroup, pid int) (cgroups.ActiveCgroup, error) {
	var (
		unitName   = c.Parent + "-" + c.Name + ".scope"
		slice      = "system.slice"
		properties []systemd1.Property
	)

	for _, v := range c.UnitProperties {
		switch v[0] {
		case "Slice":
			slice = v[1]
		default:
			return nil, fmt.Errorf("Unknown unit propery %s", v[0])
		}
	}

	properties = append(properties,
		systemd1.Property{"Slice", dbus.MakeVariant(slice)},
		systemd1.Property{"Description", dbus.MakeVariant("docker container " + c.Name)},
		systemd1.Property{"PIDs", dbus.MakeVariant([]uint32{uint32(pid)})},
	)

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

	// Always enable accounting, this gets us the same behaviour as the raw implementation,
	// plus the kernel has some problems with joining the memory cgroup at a later time.
	properties = append(properties,
		systemd1.Property{"MemoryAccounting", dbus.MakeVariant(true)},
		systemd1.Property{"CPUAccounting", dbus.MakeVariant(true)})

	if c.Memory != 0 {
		properties = append(properties,
			systemd1.Property{"MemoryLimit", dbus.MakeVariant(uint64(c.Memory))})
	}
	// TODO: MemorySwap not available in systemd

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
		mountpoint, err := cgroups.FindCgroupMountpoint("devices")
		if err != nil {
			return nil, err
		}

		path := filepath.Join(mountpoint, cgroup)

		// /dev/pts/*
		if err := ioutil.WriteFile(filepath.Join(path, "devices.allow"), []byte("c 136:* rwm"), 0700); err != nil {
			return nil, err
		}
		// tuntap
		if err := ioutil.WriteFile(filepath.Join(path, "devices.allow"), []byte("c 10:200 rwm"), 0700); err != nil {
			return nil, err
		}
	}
	return &systemdCgroup{}, nil
}

func (c *systemdCgroup) Cleanup() error {
	// systemd cleans up, we don't need to do anything
	return nil
}
