package swarm

import (
	"time"

	"github.com/docker/engine-api/types/mount"
)

// ContainerSpec represents the spec of a container.
type ContainerSpec struct {
	Image           string            `json:",omitempty"`
	Labels          map[string]string `json:",omitempty"`
	Command         []string          `json:",omitempty"`
	Args            []string          `json:",omitempty"`
	Env             []string          `json:",omitempty"`
	Dir             string            `json:",omitempty"`
	User            string            `json:",omitempty"`
	Groups          []string          `json:",omitempty"`
	TTY             bool              `json:",omitempty"`
	Mounts          []mount.Mount     `json:",omitempty"`
	StopGracePeriod *time.Duration    `json:",omitempty"`
}
