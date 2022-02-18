package types // import "github.com/moby/moby/libcontainerd/types"

import (
	"time"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Summary is not used on linux
type Summary struct{}

// Stats holds metrics properties as returned by containerd
type Stats struct {
	Read time.Time
	// Metrics is expected to be either one of:
	// * github.com/containerd/cgroups/stats/v1.Metrics
	// * github.com/containerd/cgroups/stats/v2.Metrics
	Metrics interface{}
}

// InterfaceToStats returns a stats object from the platform-specific interface.
func InterfaceToStats(read time.Time, v interface{}) *Stats {
	return &Stats{
		Metrics: v,
		Read:    read,
	}
}

// Resources defines updatable container resource values. TODO: it must match containerd upcoming API
type Resources specs.LinuxResources

// Checkpoints contains the details of a checkpoint
type Checkpoints struct{}
