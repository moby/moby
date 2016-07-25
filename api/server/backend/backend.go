package backend

import (
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/daemon/cluster"
)

// Backend is a facade for daemon.Daemon and cluster.Cluster.
type Backend struct {
	*daemon.Daemon
	clusterProvider *cluster.Cluster
}

// New creates a new Backend instance.
func New(d *daemon.Daemon, c *cluster.Cluster) *Backend {
	return &Backend{
		Daemon:          d,
		clusterProvider: c,
	}
}
