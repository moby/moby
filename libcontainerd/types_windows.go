package libcontainerd

import "github.com/docker/docker/libcontainerd/windowsoci"

// Spec is the base configuration for the container.
type Spec windowsoci.WindowsSpec

// Process contains information to start a specific application inside the container.
type Process windowsoci.Process

// User specifies user information for the containers main process.
type User windowsoci.User

// Summary container a container summary from containerd
type Summary struct {
	Pid     uint32
	Command string
}

// Stats contains a stats properties from containerd.
type Stats struct{}

// Resources defines updatable container resource values.
type Resources struct{}
