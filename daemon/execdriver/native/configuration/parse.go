package configuration

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/units"
	"github.com/docker/libcontainer"
)

type Action func(*libcontainer.Config, interface{}, string) error

var actions = map[string]Action{
	"cap.add":  addCap,  // add a cap
	"cap.drop": dropCap, // drop a cap

	"ns.add":  addNamespace,  // add a namespace
	"ns.drop": dropNamespace, // drop a namespace when cloning

	"net.join": joinNetNamespace, // join another containers net namespace

	"cgroups.cpu_shares":         cpuShares,         // set the cpu shares
	"cgroups.memory":             memory,            // set the memory limit
	"cgroups.memory_reservation": memoryReservation, // set the memory reservation
	"cgroups.memory_swap":        memorySwap,        // set the memory swap limit
	"cgroups.cpuset.cpus":        cpusetCpus,        // set the cpus used

	"systemd.slice": systemdSlice, // set parent Slice used for systemd unit

	"apparmor_profile": apparmorProfile, // set the apparmor profile to apply

	"fs.readonly": readonlyFs, // make the rootfs of the container read only
}

func cpusetCpus(container *libcontainer.Config, context interface{}, value string) error {
	if container.Cgroups == nil {
		return fmt.Errorf("cannot set cgroups when they are disabled")
	}
	container.Cgroups.CpusetCpus = value

	return nil
}

func systemdSlice(container *libcontainer.Config, context interface{}, value string) error {
	if container.Cgroups == nil {
		return fmt.Errorf("cannot set slice when cgroups are disabled")
	}
	container.Cgroups.Slice = value

	return nil
}

func apparmorProfile(container *libcontainer.Config, context interface{}, value string) error {
	container.AppArmorProfile = value
	return nil
}

func cpuShares(container *libcontainer.Config, context interface{}, value string) error {
	if container.Cgroups == nil {
		return fmt.Errorf("cannot set cgroups when they are disabled")
	}
	v, err := strconv.ParseInt(value, 10, 0)
	if err != nil {
		return err
	}
	container.Cgroups.CpuShares = v
	return nil
}

func memory(container *libcontainer.Config, context interface{}, value string) error {
	if container.Cgroups == nil {
		return fmt.Errorf("cannot set cgroups when they are disabled")
	}

	v, err := units.RAMInBytes(value)
	if err != nil {
		return err
	}
	container.Cgroups.Memory = v
	return nil
}

func memoryReservation(container *libcontainer.Config, context interface{}, value string) error {
	if container.Cgroups == nil {
		return fmt.Errorf("cannot set cgroups when they are disabled")
	}

	v, err := units.RAMInBytes(value)
	if err != nil {
		return err
	}
	container.Cgroups.MemoryReservation = v
	return nil
}

func memorySwap(container *libcontainer.Config, context interface{}, value string) error {
	if container.Cgroups == nil {
		return fmt.Errorf("cannot set cgroups when they are disabled")
	}
	v, err := strconv.ParseInt(value, 0, 64)
	if err != nil {
		return err
	}
	container.Cgroups.MemorySwap = v
	return nil
}

func addCap(container *libcontainer.Config, context interface{}, value string) error {
	container.Capabilities = append(container.Capabilities, value)
	return nil
}

func dropCap(container *libcontainer.Config, context interface{}, value string) error {
	// If the capability is specified multiple times, remove all instances.
	for i, capability := range container.Capabilities {
		if capability == value {
			container.Capabilities = append(container.Capabilities[:i], container.Capabilities[i+1:]...)
		}
	}

	// The capability wasn't found so we will drop it anyways.
	return nil
}

func addNamespace(container *libcontainer.Config, context interface{}, value string) error {
	container.Namespaces[value] = true
	return nil
}

func dropNamespace(container *libcontainer.Config, context interface{}, value string) error {
	container.Namespaces[value] = false
	return nil
}

func readonlyFs(container *libcontainer.Config, context interface{}, value string) error {
	switch value {
	case "1", "true":
		container.MountConfig.ReadonlyFs = true
	default:
		container.MountConfig.ReadonlyFs = false
	}
	return nil
}

func joinNetNamespace(container *libcontainer.Config, context interface{}, value string) error {
	var (
		running = context.(map[string]*exec.Cmd)
		cmd     = running[value]
	)

	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("%s is not a valid running container to join", value)
	}

	nspath := filepath.Join("/proc", fmt.Sprint(cmd.Process.Pid), "ns", "net")
	container.Networks = append(container.Networks, &libcontainer.Network{
		Type:   "netns",
		NsPath: nspath,
	})

	return nil
}

// configureCustomOptions takes string commands from the user and allows modification of the
// container's default configuration.
//
// TODO: this can be moved to a general utils or parser in pkg
func ParseConfiguration(container *libcontainer.Config, running map[string]*exec.Cmd, opts []string) error {
	for _, opt := range opts {
		kv := strings.SplitN(opt, "=", 2)
		if len(kv) < 2 {
			return fmt.Errorf("invalid format for %s", opt)
		}

		action, exists := actions[kv[0]]
		if !exists {
			return fmt.Errorf("%s is not a valid option for the native driver", kv[0])
		}

		if err := action(container, running, kv[1]); err != nil {
			return err
		}
	}
	return nil
}
