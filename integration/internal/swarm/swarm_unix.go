// +build !windows

package swarm

import (
	"testing"

	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/docker/testutil/environment"
	"gotest.tools/v3/skip"
)

// NewSwarm creates a swarm daemon for testing
func NewSwarm(t *testing.T, testEnv *environment.Execution, ops ...daemon.Option) *daemon.Daemon {
	t.Helper()
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support Swarm-mode")
	if testEnv.DaemonInfo.ExperimentalBuild {
		ops = append(ops, daemon.WithExperimental())
	}
	d := daemon.New(t, ops...)
	d.StartAndSwarmInit(t)
	return d
}
