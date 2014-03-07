package native

import (
	"fmt"
	"github.com/dotcloud/docker/execdriver"
	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/mount"
	"path/filepath"
)

// createContainer populates and configures the container type with the
// data provided by the execdriver.Command
func createContainer(c *execdriver.Command) (*libcontainer.Container, error) {
	container := getDefaultTemplate()

	container.Hostname = getEnv("HOSTNAME", c.Env)
	container.Tty = c.Tty
	container.User = c.User
	container.WorkingDir = c.WorkingDir
	container.Env = c.Env

	if c.Network != nil {
		container.Networks = []*libcontainer.Network{
			{
				Mtu:     c.Network.Mtu,
				Address: fmt.Sprintf("%s/%d", c.Network.IPAddress, c.Network.IPPrefixLen),
				Gateway: c.Network.Gateway,
				Type:    "veth",
				Context: libcontainer.Context{
					"prefix": "veth",
					"bridge": c.Network.Bridge,
				},
			},
		}
	}

	container.Cgroups.Name = c.ID
	if c.Privileged {
		container.Capabilities = nil
		container.Cgroups.DeviceAccess = true
		container.Context["apparmor_profile"] = "unconfined"
	}
	if c.Resources != nil {
		container.Cgroups.CpuShares = c.Resources.CpuShares
		container.Cgroups.Memory = c.Resources.Memory
		container.Cgroups.MemorySwap = c.Resources.MemorySwap
	}

	// because the rootfs mount point can be mounted as aufs, or other, a better way
	// to detect if we are running on a ramdisk is to look at the rootfs's parent's
	// filesystem type
	isRamdisk, err := isRootInRamdisk(filepath.Dir(c.Rootfs))
	if err != nil {
		return nil, err
	}
	// so when we are running on ramdisk do not pivot root
	container.NoPivotRoot = isRamdisk

	return container, nil
}

// getDefaultTemplate returns the docker default for
// the libcontainer configuration file
func getDefaultTemplate() *libcontainer.Container {
	return &libcontainer.Container{
		Capabilities: libcontainer.Capabilities{
			libcontainer.GetCapability("SETPCAP"),
			libcontainer.GetCapability("SYS_MODULE"),
			libcontainer.GetCapability("SYS_RAWIO"),
			libcontainer.GetCapability("SYS_PACCT"),
			libcontainer.GetCapability("SYS_ADMIN"),
			libcontainer.GetCapability("SYS_NICE"),
			libcontainer.GetCapability("SYS_RESOURCE"),
			libcontainer.GetCapability("SYS_TIME"),
			libcontainer.GetCapability("SYS_TTY_CONFIG"),
			libcontainer.GetCapability("MKNOD"),
			libcontainer.GetCapability("AUDIT_WRITE"),
			libcontainer.GetCapability("AUDIT_CONTROL"),
			libcontainer.GetCapability("MAC_OVERRIDE"),
			libcontainer.GetCapability("MAC_ADMIN"),
			libcontainer.GetCapability("NET_ADMIN"),
		},
		Namespaces: libcontainer.Namespaces{
			libcontainer.GetNamespace("NEWNS"),
			libcontainer.GetNamespace("NEWUTS"),
			libcontainer.GetNamespace("NEWIPC"),
			libcontainer.GetNamespace("NEWPID"),
			libcontainer.GetNamespace("NEWNET"),
		},
		Cgroups: &cgroups.Cgroup{
			Parent:       "docker",
			DeviceAccess: false,
		},
		Context: libcontainer.Context{
			"apparmor_profile": "docker-default",
		},
	}
}

func isRootInRamdisk(rootfs string) (bool, error) {
	fsType, err := mount.FindMountType(rootfs)
	if err != nil {
		return false, err
	}
	return fsType == "tmpfs" || fsType == "rootfs" || fsType == "ramfs", nil
}
