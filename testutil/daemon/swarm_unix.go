// +build !windows

package daemon

import (
	"testing"

	"github.com/docker/docker/api/types/swarm"
)

// StartAndSwarmInit starts the daemon (with busybox) and init the swarm
func (d *Daemon) StartAndSwarmInit(t testing.TB) {
	d.StartNodeWithBusybox(t)
	d.SwarmInit(t, swarm.InitRequest{})
}
