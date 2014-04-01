package configuration

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/utils"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Action func(*libcontainer.Container, interface{}, string) error

var actions = map[string]Action{
	"cap.add":  addCap,  // add a cap
	"cap.drop": dropCap, // drop a cap

	"ns.add":  addNamespace,  // add a namespace
	"ns.drop": dropNamespace, // drop a namespace when cloning

	"net.join": joinNetNamespace, // join another containers net namespace

	"cgroups.cpu_shares":  cpuShares,  // set the cpu shares
	"cgroups.memory":      memory,     // set the memory limit
	"cgroups.memory_swap": memorySwap, // set the memory swap limit
	"cgroups.cpuset.cpus": cpusetCpus, // set the cpus used

	"apparmor_profile": apparmorProfile, // set the apparmor profile to apply

	"fs.readonly": readonlyFs, // make the rootfs of the container read only
}

func cpusetCpus(container *libcontainer.Container, context interface{}, value string) error {
	if container.Cgroups == nil {
		return fmt.Errorf("cannot set cgroups when they are disabled")
	}
	container.Cgroups.CpusetCpus = value

	return nil
}

func apparmorProfile(container *libcontainer.Container, context interface{}, value string) error {
	container.Context["apparmor_profile"] = value
	return nil
}

func cpuShares(container *libcontainer.Container, context interface{}, value string) error {
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

func memory(container *libcontainer.Container, context interface{}, value string) error {
	if container.Cgroups == nil {
		return fmt.Errorf("cannot set cgroups when they are disabled")
	}

	v, err := utils.RAMInBytes(value)
	if err != nil {
		return err
	}
	container.Cgroups.Memory = v
	return nil
}

func memorySwap(container *libcontainer.Container, context interface{}, value string) error {
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

func addCap(container *libcontainer.Container, context interface{}, value string) error {
	c := container.CapabilitiesMask.Get(value)
	if c == nil {
		return fmt.Errorf("%s is not a valid capability", value)
	}
	c.Enabled = true
	return nil
}

func dropCap(container *libcontainer.Container, context interface{}, value string) error {
	c := container.CapabilitiesMask.Get(value)
	if c == nil {
		return fmt.Errorf("%s is not a valid capability", value)
	}
	c.Enabled = false
	return nil
}

func addNamespace(container *libcontainer.Container, context interface{}, value string) error {
	ns := container.Namespaces.Get(value)
	if ns == nil {
		return fmt.Errorf("%s is not a valid namespace", value[1:])
	}
	ns.Enabled = true
	return nil
}

func dropNamespace(container *libcontainer.Container, context interface{}, value string) error {
	ns := container.Namespaces.Get(value)
	if ns == nil {
		return fmt.Errorf("%s is not a valid namespace", value[1:])
	}
	ns.Enabled = false
	return nil
}

func readonlyFs(container *libcontainer.Container, context interface{}, value string) error {
	switch value {
	case "1", "true":
		container.ReadonlyFs = true
	default:
		container.ReadonlyFs = false
	}
	return nil
}

func joinNetNamespace(container *libcontainer.Container, context interface{}, value string) error {
	var (
		running = context.(map[string]*exec.Cmd)
		cmd     = running[value]
	)

	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("%s is not a valid running container to join", value)
	}
	nspath := filepath.Join("/proc", fmt.Sprint(cmd.Process.Pid), "ns", "net")
	container.Networks = append(container.Networks, &libcontainer.Network{
		Type: "netns",
		Context: libcontainer.Context{
			"nspath": nspath,
		},
	})
	return nil
}

func vethMacAddress(container *libcontainer.Container, context interface{}, value string) error {
	var veth *libcontainer.Network
	for _, network := range container.Networks {
		if network.Type == "veth" {
			veth = network
			break
		}
	}
	if veth == nil {
		return fmt.Errorf("not veth configured for container")
	}
	veth.Context["mac"] = value
	return nil
}

// configureCustomOptions takes string commands from the user and allows modification of the
// container's default configuration.
//
// TODO: this can be moved to a general utils or parser in pkg
func ParseConfiguration(container *libcontainer.Container, running map[string]*exec.Cmd, opts []string) error {
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
