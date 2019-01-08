package types // import "github.com/docker/docker/libcontainerd/types"

import (
	"time"

	"github.com/Microsoft/hcsshim"
)

// Summary contains a ProcessList item from HCS to support `top`
type Summary hcsshim.ProcessListItem

// Stats contains statistics from HCS
type Stats struct {
	Read     time.Time
	HCSStats *hcsshim.Statistics
}

// InterfaceToStats returns a stats object from the platform-specific interface.
func InterfaceToStats(read time.Time, v interface{}) *Stats {
	return &Stats{
		HCSStats: v.(*hcsshim.Statistics),
		Read:     read,
	}
}

// Resources defines updatable container resource values.
type Resources struct{}

// Checkpoint holds the details of a checkpoint (not supported in windows)
type Checkpoint struct {
	Name string
}

// Checkpoints contains the details of a checkpoint
type Checkpoints struct {
	Checkpoints []*Checkpoint
}
