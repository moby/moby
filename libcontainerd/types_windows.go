package libcontainerd // import "github.com/docker/docker/libcontainerd"

import (
	"time"

	"github.com/Microsoft/hcsshim"
	opengcs "github.com/Microsoft/opengcs/client"
)

// Summary contains a ProcessList item from HCS to support `top`
type Summary hcsshim.ProcessListItem

// Stats contains statistics from HCS
type Stats struct {
	Read     time.Time
	HCSStats *hcsshim.Statistics
}

func interfaceToStats(read time.Time, v interface{}) *Stats {
	return &Stats{
		HCSStats: v.(*hcsshim.Statistics),
		Read:     read,
	}
}

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
