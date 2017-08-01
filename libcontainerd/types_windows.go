package libcontainerd

import (
	"github.com/Microsoft/hcsshim"
	opengcs "github.com/Microsoft/opengcs/client"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// Process contains information to start a specific application inside the container.
type Process specs.Process

// Summary contains a ProcessList item from HCS to support `top`
type Summary hcsshim.ProcessListItem

// StateInfo contains description about the new state container has entered.
type StateInfo struct {
	CommonStateInfo

	// Platform specific StateInfo
	UpdatePending bool // Indicates that there are some update operations pending that should be completed by a servicing container.
}

// Stats contains statistics from HCS
type Stats hcsshim.Statistics

// Resources defines updatable container resource values.
type Resources struct{}

// LCOWOption is a CreateOption required for LCOW configuration
type LCOWOption struct {
	Config *opengcs.Config
}

// Checkpoint holds the details of a checkpoint (not supported in windows)
type Checkpoint struct {
	Name string
}

// Checkpoints contains the details of a checkpoint
type Checkpoints struct {
	Checkpoints []*Checkpoint
}
