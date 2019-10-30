package types // import "github.com/docker/docker/libcontainerd/types"

import (
	"time"

	v1 "github.com/containerd/cgroups/stats/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Summary is not used on linux
type Summary struct{}

// Stats holds metrics properties as returned by containerd
type Stats struct {
	Read    time.Time
	Metrics *v1.Metrics
}

// InterfaceToStats returns a stats object from the platform-specific interface.
func InterfaceToStats(read time.Time, v interface{}) *Stats {
	return &Stats{
		Metrics: v.(*v1.Metrics),
		Read:    read,
	}
}

// Resources defines updatable container resource values. TODO: it must match containerd upcoming API
type Resources specs.LinuxResources

// Checkpoints contains the details of a checkpoint
type Checkpoints struct{}
