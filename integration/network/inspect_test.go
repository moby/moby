package network // import "github.com/docker/docker/integration/network"

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/integration/internal/swarm"
	"gotest.tools/assert"
	"gotest.tools/poll"
	"gotest.tools/skip"
)

func TestInspectNetwork(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows", "FIXME")
	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	networkName := "Overlay" + t.Name()
	overlayID := network.CreateNoError(t, context.Background(), c, networkName,
		network.WithDriver("overlay"),
		network.WithCheckDuplicate(),
	)

	var instances uint64 = 2
	serviceName := "TestService" + t.Name()

	serviceID := swarm.CreateService(t, d,
		swarm.ServiceWithReplicas(instances),
		swarm.ServiceWithName(serviceName),
		swarm.ServiceWithNetwork(networkName),
	)

	poll.WaitOn(t, serviceRunningTasksCount(c, serviceID, instances), swarm.ServicePoll)

	tests := []struct {
		name    string
		network string
		opts    types.NetworkInspectOptions
	}{
		{
			name:    "full network id",
			network: overlayID,
			opts: types.NetworkInspectOptions{
				Verbose: true,
			},
		},
		{
			name:    "partial network id",
			network: overlayID[0:11],
			opts: types.NetworkInspectOptions{
				Verbose: true,
			},
		},
		{
			name:    "network name",
			network: networkName,
			opts: types.NetworkInspectOptions{
				Verbose: true,
			},
		},
		{
			name:    "network name and swarm scope",
			network: networkName,
			opts: types.NetworkInspectOptions{
				Verbose: true,
				Scope:   "swarm",
			},
		},
	}
	ctx := context.Background()
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
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

func serviceRunningTasksCount(client client.ServiceAPIClient, serviceID string, instances uint64) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		tasks, err := client.TaskList(context.Background(), types.TaskListOptions{
			Filters: filters.NewArgs(
				filters.Arg("service", serviceID),
				filters.Arg("desired-state", string(swarmtypes.TaskStateRunning)),
			),
		})
		switch {
		case err != nil:
			return poll.Error(err)
		case len(tasks) == int(instances):
			for _, task := range tasks {
				if task.Status.Err != "" {
					log.Log("task error:", task.Status.Err)
				}
				if task.Status.State != swarmtypes.TaskStateRunning {
					return poll.Continue("waiting for tasks to enter run state (current status: %s)", task.Status.State)
				}
			}
			return poll.Success()
		default:
			return poll.Continue("task count for service %s at %d waiting for %d", serviceID, len(tasks), instances)
		}
	}
}
