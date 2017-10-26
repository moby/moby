package libcontainerd

import (
	"time"

	"github.com/containerd/cgroups"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Summary is not used on linux
type Summary struct{}

// Stats holds metrics properties as returned by containerd
type Stats struct {
	Read    time.Time
	Metrics *cgroups.Metrics
}

func interfaceToStats(read time.Time, v interface{}) *Stats {
	return &Stats{
		Metrics: v.(*cgroups.Metrics),
		Read:    read,
	}
}

// Resources defines updatable container resource values. TODO: it must match containerd upcoming API
type Resources specs.LinuxResources

// Checkpoints contains the details of a checkpoint
type Checkpoints struct{}
