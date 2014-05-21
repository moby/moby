package libcontainer

import (
	"github.com/dotcloud/docker/pkg/libcontainer/cgroups"
)

// Context is a generic key value pair that allows arbatrary data to be sent
type Context map[string]string

// Container defines configuration options for executing a process inside a contained environment
type Container struct {
	// Hostname optionally sets the container's hostname if provided
	Hostname string `json:"hostname,omitempty"`

	// ReadonlyFs will remount the container's rootfs as readonly where only externally mounted
	// bind mounts are writtable
	ReadonlyFs bool `json:"readonly_fs,omitempty"`

	// NoPivotRoot will use MS_MOVE and a chroot to jail the process into the container's rootfs
	// This is a common option when the container is running in ramdisk
	NoPivotRoot bool `json:"no_pivot_root,omitempty"`

	// User will set the uid and gid of the executing process running inside the container
	User string `json:"user,omitempty"`

	// WorkingDir will change the processes current working directory inside the container's rootfs
	WorkingDir string `json:"working_dir,omitempty"`

	// Env will populate the processes environment with the provided values
	// Any values from the parent processes will be cleared before the values
	// provided in Env are provided to the process
	Env []string `json:"environment,omitempty"`

	// Tty when true will allocate a pty slave on the host for access by the container's process
	// and ensure that it is mounted inside the container's rootfs
	Tty bool `json:"tty,omitempty"`

	// Namespaces specifies the container's namespaces that it should setup when cloning the init process
	// If a namespace is not provided that namespace is shared from the container's parent process
	Namespaces map[string]bool `json:"namespaces,omitempty"`

	// Capabilities specify the capabilities to keep when executing the process inside the container
	// All capbilities not specified will be dropped from the processes capability mask
	Capabilities []string `json:"capabilities,omitempty"`

	// Networks specifies the container's network setup to be created
	Networks []*Network `json:"networks,omitempty"`

	// Cgroups specifies specific cgroup settings for the various subsystems that the container is
	// placed into to limit the resources the container has available
	Cgroups *cgroups.Cgroup `json:"cgroups,omitempty"`

	// Context is a generic key value format that allows for additional settings to be passed
	// on the container's creation
	// This is commonly used to specify apparmor profiles, selinux labels, and different restrictions
	// placed on the container's processes
	Context Context `json:"context,omitempty"`

	// Mounts specify additional source and destination paths that will be mounted inside the container's
	// rootfs and mount namespace if specified
	Mounts Mounts `json:"mounts,omitempty"`

	// RequiredDeviceNodes are a list of device nodes that will be mknod into the container's rootfs at /dev
	// If the host system does not support the device that the container requests an error is returned
	RequiredDeviceNodes []string `json:"required_device_nodes,omitempty"`

	// OptionalDeviceNodes are a list of device nodes that will be mknod into the container's rootfs at /dev
	// If the host system does not support the device that the container requests the error is ignored
	OptionalDeviceNodes []string `json:"optional_device_nodes,omitempty"`
}

// Network defines configuration for a container's networking stack
//
// The network configuration can be omited from a container causing the
// container to be setup with the host's networking stack
type Network struct {
	// Type sets the networks type, commonly veth and loopback
	Type string `json:"type,omitempty"`

	// Context is a generic key value format for setting additional options that are specific to
	// the network type
	Context Context `json:"context,omitempty"`

	// Address contains the IP and mask to set on the network interface
	Address string `json:"address,omitempty"`

	// Gateway sets the gateway address that is used as the default for the interface
	Gateway string `json:"gateway,omitempty"`

	// Mtu sets the mtu value for the interface and will be mirrored on both the host and
	// container's interfaces if a pair is created, specifically in the case of type veth
	Mtu int `json:"mtu,omitempty"`
}
