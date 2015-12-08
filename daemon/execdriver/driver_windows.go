package execdriver

import (
	"github.com/docker/docker/pkg/nat"
	"github.com/docker/docker/runconfig"
)

// Mount contains information for a mount operation.
type Mount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Writable    bool   `json:"writable"`
}

// Resources contains all resource configs for a driver.
// Currently these are all for cgroup configs.
type Resources struct {
	CommonResources

	// Fields below here are platform specific
}

// ProcessConfig is the platform specific structure that describes a process
// that will be run inside a container.
type ProcessConfig struct {
	CommonProcessConfig

	// Fields below here are platform specific
	ConsoleSize [2]int `json:"-"` // h,w of initial console size
}

// Network settings of the container
type Network struct {
	Interface   *NetworkInterface `json:"interface"`
	ContainerID string            `json:"container_id"` // id of the container to join network.
}

// NetworkInterface contains network configs for a driver
type NetworkInterface struct {
	MacAddress string `json:"mac"`
	Bridge     string `json:"bridge"`
	IPAddress  string `json:"ip"`

	// PortBindings is the port mapping between the exposed port in the
	// container and the port on the host.
	PortBindings nat.PortMap `json:"port_bindings"`
}

// Command wraps an os/exec.Cmd to add more metadata
type Command struct {
	CommonCommand

	// Fields below here are platform specific

	FirstStart  bool                     `json:"first_start"`  // Optimisation for first boot of Windows
	Hostname    string                   `json:"hostname"`     // Windows sets the hostname in the execdriver
	LayerFolder string                   `json:"layer_folder"` // Layer folder for a command
	LayerPaths  []string                 `json:"layer_paths"`  // Layer paths for a command
	Isolation   runconfig.IsolationLevel `json:"isolation"`    // Isolation level for the container
	ArgsEscaped bool                     `json:"args_escaped"` // True if args are already escaped
}

// ExitStatus provides exit reasons for a container.
type ExitStatus struct {
	// The exit code with which the container exited.
	ExitCode int
}
