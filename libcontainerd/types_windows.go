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

// StateInfo contains description about the new state container has entered.
type StateInfo struct {
	CommonStateInfo

	// Platform specific StateInfo

	UpdatePending bool // Indicates that there are some update operations pending that should be completed by a servicing container.
}

// Stats contains a stats properties from containerd.
type Stats struct{}

// Resources defines updatable container resource values.
type Resources struct{}

// ServicingOption is an empty CreateOption with a no-op application that siginifies
// the container needs to be use for a Windows servicing operation.
type ServicingOption struct {
	IsServicing bool
}
