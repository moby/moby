package stats

import (
	// go mod will not vendor without an import for metrics.proto
	_ "github.com/containerd/cgroups/v3/cgroup1/stats"
)
