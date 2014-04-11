package libcontainer

import (
	"github.com/dotcloud/docker/pkg/cgroups"
)

// Context is a generic key value pair that allows
// arbatrary data to be sent
type Context map[string]string

// Container defines configuration options for how a
// container is setup inside a directory and how a process should be executed
type Container struct {
	Hostname         string          `json:"hostname,omitempty"`          // hostname
	ReadonlyFs       bool            `json:"readonly_fs,omitempty"`       // set the containers rootfs as readonly
	NoPivotRoot      bool            `json:"no_pivot_root,omitempty"`     // this can be enabled if you are running in ramdisk
	User             string          `json:"user,omitempty"`              // user to execute the process as
	WorkingDir       string          `json:"working_dir,omitempty"`       // current working directory
	Env              []string        `json:"environment,omitempty"`       // environment to set
	Tty              bool            `json:"tty,omitempty"`               // setup a proper tty or not
	Namespaces       Namespaces      `json:"namespaces,omitempty"`        // namespaces to apply
	CapabilitiesMask Capabilities    `json:"capabilities_mask,omitempty"` // capabilities to drop
	Networks         []*Network      `json:"networks,omitempty"`          // nil for host's network stack
	Cgroups          *cgroups.Cgroup `json:"cgroups,omitempty"`           // cgroups
	Context          Context         `json:"context,omitempty"`           // generic context for specific options (apparmor, selinux)
	Mounts           Mounts          `json:"mounts,omitempty"`
}

// Network defines configuration for a container's networking stack
//
// The network configuration can be omited from a container causing the
// container to be setup with the host's networking stack
type Network struct {
	Type    string  `json:"type,omitempty"`    // type of networking to setup i.e. veth, macvlan, etc
	Context Context `json:"context,omitempty"` // generic context for type specific networking options
	Address string  `json:"address,omitempty"`
	Gateway string  `json:"gateway,omitempty"`
	Mtu     int     `json:"mtu,omitempty"`
}

type Mounts []Mount

func (s Mounts) OfType(t string) Mounts {
	out := Mounts{}
	for _, m := range s {
		if m.Type == t {
			out = append(out, m)
		}
	}
	return out
}

type Mount struct {
	Type        string `json:"type,omitempty"`
	Source      string `json:"source,omitempty"`      // Source path, in the host namespace
	Destination string `json:"destination,omitempty"` // Destination path, in the container
	Writable    bool   `json:"writable,omitempty"`
	Private     bool   `json:"private,omitempty"`
}
