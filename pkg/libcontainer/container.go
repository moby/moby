package libcontainer

// Container defines configuration options for how a
// container is setup inside a directory and how a process should be executed
type Container struct {
	Hostname     string       `json:"hostname,omitempty"`     // hostname
	ReadonlyFs   bool         `json:"readonly_fs,omitempty"`  // set the containers rootfs as readonly
	User         string       `json:"user,omitempty"`         // user to execute the process as
	WorkingDir   string       `json:"working_dir,omitempty"`  // current working directory
	Env          []string     `json:"environment,omitempty"`  // environment to set
	Namespaces   Namespaces   `json:"namespaces,omitempty"`   // namespaces to apply
	Capabilities Capabilities `json:"capabilities,omitempty"` // capabilities to drop
	Network      *Network     `json:"network,omitempty"`      // nil for host's network stack

	CgroupName   string `json:"cgroup_name,omitempty"`   // name of cgroup
	CgroupParent string `json:"cgroup_parent,omitempty"` // name of parent cgroup or slice
	DeviceAccess bool   `json:"device_access,omitempty"` // name of parent cgroup or slice
	Memory       int64  `json:"memory,omitempty"`        // Memory limit (in bytes)
	MemorySwap   int64  `json:"memory_swap,omitempty"`   // Total memory usage (memory + swap); set `-1' to disable swap
	CpuShares    int64  `json:"cpu_shares,omitempty"`    // CPU shares (relative weight vs. other containers)
}

// Network defines configuration for a container's networking stack
//
// The network configuration can be omited from a container causing the
// container to be setup with the host's networking stack
type Network struct {
	IP      string `json:"ip,omitempty"`
	Gateway string `json:"gateway,omitempty"`
	Bridge  string `json:"bridge,omitempty"`
	Mtu     int    `json:"mtu,omitempty"`
}
