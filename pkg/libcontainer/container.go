package libcontainer

import (
	"github.com/dotcloud/docker/pkg/cgroups"
)

// Container defines configuration options for how a
// container is setup inside a directory and how a process should be executed
type Container struct {
	Hostname     string          `json:"hostname,omitempty"`     // hostname
	ReadonlyFs   bool            `json:"readonly_fs,omitempty"`  // set the containers rootfs as readonly
	User         string          `json:"user,omitempty"`         // user to execute the process as
	WorkingDir   string          `json:"working_dir,omitempty"`  // current working directory
	Env          []string        `json:"environment,omitempty"`  // environment to set
	Namespaces   Namespaces      `json:"namespaces,omitempty"`   // namespaces to apply
	Capabilities Capabilities    `json:"capabilities,omitempty"` // capabilities to drop
	Network      *Network        `json:"network,omitempty"`      // nil for host's network stack
	Cgroups      *cgroups.Cgroup `json:"cgroups,omitempty"`
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
