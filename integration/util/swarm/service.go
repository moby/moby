package swarm

import (
	"fmt"
	"testing"

	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/docker/docker/internal/test/environment"
	"github.com/stretchr/testify/require"
)

const (
	dockerdBinary    = "dockerd"
	defaultSwarmPort = 2477
)

// NewSwarm creates a swarm daemon for testing
func NewSwarm(t *testing.T, testEnv *environment.Execution) *daemon.Swarm {
	d := &daemon.Swarm{
		Daemon: daemon.New(t, "", dockerdBinary, daemon.Config{
			Experimental: testEnv.DaemonInfo.ExperimentalBuild,
		}),
		// TODO: better method of finding an unused port
		Port: defaultSwarmPort,
	}
	// TODO: move to a NewSwarm constructor
	d.ListenAddr = fmt.Sprintf("0.0.0.0:%d", d.Port)

	// avoid networking conflicts
	args := []string{"--iptables=false", "--swarm-default-advertise-addr=lo"}
	d.StartWithBusybox(t, args...)

	require.NoError(t, d.Init(swarmtypes.InitRequest{}))
	return d
}
