package network

import (
	"testing"

	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/network"
	"github.com/moby/moby/v2/integration/internal/swarm"
	"github.com/moby/moby/v2/testutil"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestInspectNetwork(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME")
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support Swarm-mode")
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	networkName := "Overlay" + t.Name()
	overlayID := network.CreateNoError(ctx, t, c, networkName,
		network.WithDriver("overlay"),
	)

	var instances uint64 = 2
	serviceName := "TestService" + t.Name()

	serviceID := swarm.CreateService(ctx, t, d,
		swarm.ServiceWithReplicas(instances),
		swarm.ServiceWithName(serviceName),
		swarm.ServiceWithNetwork(networkName),
	)

	poll.WaitOn(t, swarm.RunningTasksCount(ctx, c, serviceID, instances), swarm.ServicePoll)

	tests := []struct {
		name    string
		network string
		opts    client.NetworkInspectOptions
	}{
		{
			name:    "full network id",
			network: overlayID,
			opts: client.NetworkInspectOptions{
				Verbose: true,
			},
		},
		{
			name:    "partial network id",
			network: overlayID[0:11],
			opts: client.NetworkInspectOptions{
				Verbose: true,
			},
		},
		{
			name:    "network name",
			network: networkName,
			opts: client.NetworkInspectOptions{
				Verbose: true,
			},
		},
		{
			name:    "network name and swarm scope",
			network: networkName,
			opts: client.NetworkInspectOptions{
				Verbose: true,
				Scope:   "swarm",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			nw, err := c.NetworkInspect(ctx, tc.network, tc.opts)
			assert.NilError(t, err)

			if service, ok := nw.Services[serviceName]; ok {
				assert.Equal(t, len(service.Tasks), int(instances))
			}

			assert.Assert(t, nw.IPAM.Config != nil)

			for _, cfg := range nw.IPAM.Config {
				assert.Assert(t, cfg.Gateway != "")
				assert.Assert(t, cfg.Subnet != "")
			}
		})
	}

	// TODO find out why removing networks is needed; other tests fail if the network is not removed, even though they run on a new daemon.
	err := c.ServiceRemove(ctx, serviceID)
	assert.NilError(t, err)
	poll.WaitOn(t, swarm.NoTasksForService(ctx, c, serviceID), swarm.ServicePoll)
	err = c.NetworkRemove(ctx, overlayID)
	assert.NilError(t, err)
	poll.WaitOn(t, network.IsRemoved(ctx, c, overlayID), swarm.NetworkPoll)
}
