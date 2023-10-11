package types // import "github.com/docker/docker/libcontainerd/types"

import (
	"time"
)

type Summary struct{}

type Stats struct {
	Read time.Time
	// Metrics is expected to be either one of:
	// * github.com/containerd/cgroups/v3/cgroup1/stats.Metrics
	// * github.com/containerd/cgroups/v3/cgroup2/stats.Metrics
	Metrics interface{}
}

func InterfaceToStats(read time.Time, v interface{}) *Stats {
	return &Stats{
		Read:    read,
		Metrics: v,
	}
}

type Resources struct{}

type Checkpoints struct{}
