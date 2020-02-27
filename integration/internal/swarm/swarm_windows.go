// +build windows

package swarm

import (
	"testing"

	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/docker/testutil/environment"
)

// On Windows we use swarm started by hack\ci\windows.ps1
func NewSwarm(t *testing.T, testEnv *environment.Execution, ops ...daemon.Option) *daemon.Daemon {
	t.Helper()

	d := &daemon.Daemon{}
	// d.StartAndSwarmInit(t)
	return d
}
