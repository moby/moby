package libcontainerd

import (
	"github.com/opencontainers/specs/specs-go"
)

// Spec is the base configuration for the container.  It specifies platform
// independent configuration. This information must be included when the
// bundle is packaged for distribution.
type Spec specs.Spec

// Process contains information to start a specific application inside the container.
type Process struct {
	// Terminal creates an interactive terminal for the container.
	Terminal bool `json:"terminal"`
	// Args specifies the binary and arguments for the application to execute.
	Args []string `json:"args"`
}

// Stats contains a stats properties from containerd.
type Stats struct{}

// Summary container a container summary from containerd
type Summary struct{}

// StateInfo contains description about the new state container has entered.
type StateInfo struct {
	CommonStateInfo

	// Platform specific StateInfo
}

// User specifies Solaris specific user and group information for the container's
// main process.
type User specs.User

// Resources defines updatable container resource values.
type Resources struct{}
