package libcontainerd

import (
	"github.com/Microsoft/hcsshim"
	"github.com/docker/docker/libcontainerd/windowsoci"
)

// Spec is the base configuration for the container.
type Spec windowsoci.WindowsSpec

// Process contains information to start a specific application inside the container.
type Process windowsoci.Process

// User specifies user information for the containers main process.
type User windowsoci.User

// Summary contains a ProcessList item from HCS to support `top`
type Summary hcsshim.ProcessListItem

// StateInfo contains description about the new state container has entered.
type StateInfo struct {
	CommonStateInfo

	// Platform specific StateInfo

	UpdatePending bool // Indicates that there are some update operations pending that should be completed by a servicing container.
}

// Stats contains statics from HCS
type Stats hcsshim.Statistics

// Resources defines updatable container resource values.
type Resources struct{}

// ServicingOption is an empty CreateOption with a no-op application that signifies
// the container needs to be used for a Windows servicing operation.
type ServicingOption struct {
	IsServicing bool
}

// FlushOption is an empty CreateOption that signifies if the container should be
// started with flushes ignored until boot has completed. This is an optimisation
// for first boot of a container.
type FlushOption struct {
	IgnoreFlushesDuringBoot bool
}

// Checkpoint holds the details of a checkpoint (not supported in windows)
type Checkpoint struct {
	Name string
}

// Checkpoints contains the details of a checkpoint
type Checkpoints struct {
	Checkpoints []*Checkpoint
}
